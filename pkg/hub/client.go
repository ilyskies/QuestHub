package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/philippseith/signalr"
)

type Client struct {
	connection signalr.Client
	url        string

	ctx    context.Context
	cancel context.CancelFunc

	timeout time.Duration

	logger    Logger
	connected bool

	mu sync.RWMutex

	readyHandlers      []func(ReadyStatus)
	disconnectHandlers []func(error)

	observeCancel context.CancelFunc
}

// receiver for server->client callbacks
type hubReceiver struct {
	signalr.Hub
	client *Client
}

func (r *hubReceiver) Ready(status ReadyStatus) {
	r.client.logger.Info(
		"Service ready - Version: %s, Initialized: %v",
		status.Version,
		status.Initialized,
	)

	r.client.mu.RLock()
	handlers := append([]func(ReadyStatus){}, r.client.readyHandlers...)
	r.client.mu.RUnlock()

	for _, h := range handlers {
		go h(status)
	}
}

func NewClient(url string, opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		url:                url,
		ctx:                ctx,
		cancel:             cancel,
		timeout:            30 * time.Second,
		logger:             &DefaultLogger{},
		readyHandlers:      make([]func(ReadyStatus), 0),
		disconnectHandlers: make([]func(error), 0),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return nil
	}

	creationCtx, cancel := context.WithTimeout(c.ctx, c.timeout)
	defer cancel()

	conn, err := signalr.NewHTTPConnection(
		creationCtx,
		c.url,
	)

	if err != nil {
		c.logger.Error("Failed to create SignalR connection: %v", err)
		return fmt.Errorf("failed to create connection: %w", err)
	}

	rcv := &hubReceiver{client: c}

	client, err := signalr.NewClient(
		c.ctx,
		signalr.WithConnection(conn),
		signalr.WithReceiver(rcv),

		signalr.Logger(noopSignalRLogger{}, false),
		signalr.MaximumReceiveMessageSize(10*1024*1024),
	)
	if err != nil {
		c.logger.Error("Failed to create SignalR client: %v", err)
		return fmt.Errorf("failed to create client: %w", err)
	}

	c.connection = client

	stateCh := make(chan signalr.ClientState, 8)
	c.observeCancel = c.connection.ObserveStateChanged(stateCh)

	go c.watchStates(stateCh)

	c.connection.Start()

	c.logger.Info("Connecting to Hub at %s", c.url)
	return nil
}

func (c *Client) watchStates(stateCh <-chan signalr.ClientState) {
	for state := range stateCh {
		switch state {
		case signalr.ClientConnected:
			c.mu.Lock()
			c.connected = true
			c.mu.Unlock()

			c.logger.Info("Connected to Hub")

		case signalr.ClientClosed:
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()

			err := c.connection.Err()
			if err == nil {
				err = ErrNotConnected
			}

			c.logger.Info("Disconnected from Hub: %v", err)

			c.mu.RLock()
			handlers := append([]func(error){}, c.disconnectHandlers...)
			c.mu.RUnlock()

			for _, h := range handlers {
				go h(err)
			}
		}
	}
}

func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connection == nil {
		c.connected = false
		return nil
	}

	if c.observeCancel != nil {
		c.observeCancel()
		c.observeCancel = nil
	}

	c.connection.Stop()

	c.connected = false
	c.logger.Info("Disconnected from Hub")
	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) OnReady(handler func(ReadyStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readyHandlers = append(c.readyHandlers, handler)
}

func (c *Client) OnDisconnect(handler func(error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectHandlers = append(c.disconnectHandlers, handler)
}

func (c *Client) invoke(ctx context.Context, method string, args ...interface{}) (interface{}, error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	ch := c.connection.Invoke(method, args...)

	select {
	case res := <-ch:
		if res.Error != nil {
			c.logger.Error(
				"Method %s failed: %v",
				method,
				res.Error,
			)
			return nil, fmt.Errorf(
				"%w: %s - %v",
				ErrInvokeFailed,
				method,
				res.Error,
			)
		}
		return res.Value, nil

	case <-ctx.Done():
		return nil, fmt.Errorf(
			"%w: %s - %v",
			ErrConnectionTimeout,
			method,
			ctx.Err(),
		)
	}
}

func (c *Client) unmarshalResult(result interface{}, target interface{}) error {
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}
	if err := json.Unmarshal(b, target); err != nil {
		return fmt.Errorf("failed to unmarshal result: %w", err)
	}
	return nil
}

func (c *Client) GetServiceStatus(ctx context.Context) (*ServiceStatus, error) {
	val, err := c.invoke(ctx, "GetServiceStatus")
	if err != nil {
		return nil, err
	}

	var out ServiceStatus
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDailyQuests(ctx context.Context) (map[string]BaseQuest, error) {
	val, err := c.invoke(ctx, "GetDailyQuests")
	if err != nil {
		return nil, err
	}

	var out map[string]BaseQuest
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetDailyQuest(ctx context.Context, questID string) (*BaseQuest, error) {
	if questID == "" {
		return nil, ErrInvalidQuestID
	}

	val, err := c.invoke(ctx, "GetDailyQuest", questID)
	if err != nil {
		return nil, err
	}

	var out BaseQuest
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetChallengeBundles(ctx context.Context) ([]AthenaChallengeBundle, error) {
	val, err := c.invoke(ctx, "GetChallengeBundles")
	if err != nil {
		return nil, err
	}

	var out []AthenaChallengeBundle
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetChallengeBundle(ctx context.Context, templateID string) (*AthenaChallengeBundle, error) {
	if templateID == "" {
		return nil, ErrInvalidTemplateID
	}

	val, err := c.invoke(ctx, "GetChallengeBundle", templateID)
	if err != nil {
		return nil, err
	}

	var out AthenaChallengeBundle
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetChallengeBundleSchedules(ctx context.Context) ([]ChallengeBundleSchedule, error) {
	val, err := c.invoke(ctx, "GetChallengeBundleSchedules")
	if err != nil {
		return nil, err
	}

	var out []ChallengeBundleSchedule
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) ClearCache(ctx context.Context) (*CacheResult, error) {
	val, err := c.invoke(ctx, "ClearCache")
	if err != nil {
		return nil, err
	}

	var out CacheResult
	if err := c.unmarshalResult(val, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) RefreshCache(ctx context.Context) error {
	_, err := c.invoke(ctx, "RefreshCache")
	return err
}
