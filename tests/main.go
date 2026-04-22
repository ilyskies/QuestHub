package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ilyskies/QuestHub/pkg/hub"
)

const (
	hubURL         = "http://localhost:5294/hub"
	readyTimeout   = 15 * time.Second
	defaultTimeout = 15 * time.Second
)

func main() {
	client := newClient()

	if err := connect(client); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.Disconnect()

	if err := waitForReady(client); err != nil {
		log.Printf("Ready wait failed: %v", err)
	} else {
		if err := runOperations(client); err != nil {
			log.Printf("Operations error: %v", err)
		}
	}

	waitForShutdown()
}

func newClient() *hub.Client {
	return hub.NewClient(
		hubURL,
		hub.WithTimeout(30*time.Second),
	)
}

func connect(client *hub.Client) error {
	log.Println("Connecting to Hub...")
	if err := client.Connect(); err != nil {
		return err
	}
	log.Println("Connected successfully")

	client.OnDisconnect(func(err error) {
		log.Printf("Disconnected: %v", "I miss you skies :( Me sad you gone :(", err)
	})

	return nil
}

func waitForReady(client *hub.Client) error {
	readyCh := make(chan struct{}, 1)
	var triggered atomic.Bool

	client.OnReady(func(status hub.ReadyStatus) {
		if status.Initialized && triggered.CompareAndSwap(false, true) {
			readyCh <- struct{}{}
		}
	})

	select {
	case <-readyCh:
		return nil
	case <-time.After(readyTimeout):
		return fmt.Errorf("timed out waiting for Ready")
	}
}

func runOperations(client *hub.Client) error {
	if err := logServiceStatus(client); err != nil {
		return err
	}

	if err := clearCache(client); err != nil {
		return err
	}

	if err := fetchAndSave("daily_quests.json", client.GetDailyQuests); err != nil {
		return err
	}

	if err := fetchAndSave("challenge_bundles.json", client.GetChallengeBundles); err != nil {
		return err
	}

	if err := fetchAndSave("bundle_schedules.json", client.GetChallengeBundleSchedules); err != nil {
		return err
	}

	return nil
}

func logServiceStatus(client *hub.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := client.GetServiceStatus(ctx)
	if err != nil {
		return fmt.Errorf("get status: %w", err)
	}

	log.Printf("Version: %s", status.Version)
	log.Printf("Initialized: %v", status.Initialized)
	return nil
}

func clearCache(client *hub.Client) error {
	log.Println("\nClearing cache")

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	result, err := client.ClearCache(ctx)
	if err != nil {
		return fmt.Errorf("clear cache: %w", err)
	}

	log.Printf("Cleared %d keys", result.KeysCleared)
	return nil
}

func fetchAndSave[T any](
	filename string,
	fetch func(context.Context) (T, error),
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := fetch(ctx)
	if err != nil {
		return err
	}

	if err := writeJSONFile(filename, data); err != nil {
		return err
	}

	log.Printf("Saved %s", filename)
	return nil
}

func writeJSONFile(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down...")
}