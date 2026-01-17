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

	"com.beyond.services.api.client/pkg/hub"
)

func main() {
	client := hub.NewClient(
		"http://localhost:5294/hub",
		hub.WithTimeout(30*time.Second),
	)

	readyCh := make(chan struct{}, 1)
	var ran atomic.Bool

	client.OnReady(func(status hub.ReadyStatus) {
		if status.Initialized && ran.CompareAndSwap(false, true) {
			readyCh <- struct{}{}
		}
	})

	client.OnDisconnect(func(err error) {
		log.Printf("Disconnected: %v", err)
	})

	log.Println("Connecting to Hub...")
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	log.Println("Connected successfully")

	select {
	case <-readyCh:
		if err := runOperations(client); err != nil {
			log.Printf("Error running operations: %v", err)
		}
	case <-time.After(15 * time.Second):
		log.Printf("Timed out waiting for Ready")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
}

func runOperations(client *hub.Client) error {
	call := func(d time.Duration) (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), d)
	}

	{
		ctx, cancel := call(10 * time.Second)
		defer cancel()
		status, err := client.GetServiceStatus(ctx)
		if err != nil {
			return fmt.Errorf("get status: %w", err)
		}
		log.Printf("Version: %s", status.Version)
		log.Printf("Initialized: %v", status.Initialized)
	}

	log.Println("\nClearing cache")
	{
		ctx, cancel := call(15 * time.Second)
		defer cancel()
		clearResult, err := client.ClearCache(ctx)
		if err != nil {
			return fmt.Errorf("clear cache: %w", err)
		}
		log.Printf("Cleared %d keys", clearResult.KeysCleared)
	}

	{
		ctx, cancel := call(15 * time.Second)
		defer cancel()
		quests, err := client.GetDailyQuests(ctx)
		if err != nil {
			return fmt.Errorf("get quests: %w", err)
		}

		if err := writeJSONFile("daily_quests.json", quests); err != nil {
			return fmt.Errorf("write daily quests json: %w", err)
		}

		log.Printf("Found %d daily quests", len(quests))
	}

	{
		ctx, cancel := call(30 * time.Second)
		defer cancel()
		bundles, err := client.GetChallengeBundles(ctx)
		if err != nil {
			return fmt.Errorf("get bundles: %w", err)
		}

		if err := writeJSONFile("challenge_bundles.json", bundles); err != nil {
			return fmt.Errorf("write challenge bundles json: %w", err)
		}

		log.Printf("Found %d challenge bundles", len(bundles))
	}

	{
		ctx, cancel := call(30 * time.Second)
		defer cancel()
		schedules, err := client.GetChallengeBundleSchedules(ctx)
		if err != nil {
			return fmt.Errorf("get schedules: %w", err)
		}

		if err := writeJSONFile("bundle_schedules.json", schedules); err != nil {
			return fmt.Errorf("write schedules json: %w", err)
		}

		log.Printf("Found %d schedules", len(schedules))
	}

	return nil
}

func writeJSONFile(filename string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}
