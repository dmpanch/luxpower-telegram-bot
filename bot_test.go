package main

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func testDSN(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
}

type fakeSender struct {
	mu       sync.Mutex
	messages map[int64][]string
}

func newFakeSender() *fakeSender {
	return &fakeSender{messages: make(map[int64][]string)}
}

func (f *fakeSender) Send(chatID int64, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages[chatID] = append(f.messages[chatID], message)
	return nil
}

func (f *fakeSender) SentTo(chatID int64) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.messages[chatID]...)
}

func TestBotLoadsSubscribersOnInit(t *testing.T) {
	store, err := NewSubscriberStore(testDSN(t))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	wantIDs := []int64{101, 202}
	for _, id := range wantIDs {
		if err := store.Save(id); err != nil {
			t.Fatalf("failed to seed id %d: %v", id, err)
		}
	}

	bot := &Bot{
		store:            store,
		chatIDs:          make(map[int64]bool),
		sendMessage:      func(int64, string) error { return nil },
		fetchGridState:   func() (int, error) { return 1, nil },
		lastNotification: make(map[notificationType]time.Time),
	}

	if err := bot.loadSubscribers(); err != nil {
		t.Fatalf("loadSubscribers returned error: %v", err)
	}

	for _, id := range wantIDs {
		if !bot.chatIDs[id] {
			t.Fatalf("chat ID %d not loaded into bot state", id)
		}
	}
}

func TestBotRegisterChatIDPersistsAndDeduplicates(t *testing.T) {
	store, err := NewSubscriberStore(testDSN(t))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	bot := &Bot{
		store:            store,
		chatIDs:          make(map[int64]bool),
		sendMessage:      func(int64, string) error { return nil },
		fetchGridState:   func() (int, error) { return 1, nil },
		lastNotification: make(map[notificationType]time.Time),
	}

	bot.registerChatID(777)
	bot.registerChatID(777) // duplicate

	ids, err := store.List()
	if err != nil {
		t.Fatalf("store.List failed: %v", err)
	}
	if len(ids) != 1 || ids[0] != 777 {
		t.Fatalf("expected single stored id 777, got %v", ids)
	}
}

func TestBroadcastInitialStatusSendsCurrentState(t *testing.T) {
	store, err := NewSubscriberStore(testDSN(t))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	if err := store.Save(1); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := store.Save(2); err != nil {
		t.Fatalf("seed store: %v", err)
	}

	sender := newFakeSender()

	bot := &Bot{
		store:             store,
		chatIDs:           make(map[int64]bool),
		sendMessage:       sender.Send,
		fetchGridState:    func() (int, error) { return 0, nil },
		currentGridState:  -1,
		previousGridState: -1,
		lastNotification:  make(map[notificationType]time.Time),
	}

	if err := bot.loadSubscribers(); err != nil {
		t.Fatalf("loadSubscribers returned error: %v", err)
	}

	bot.broadcastInitialStatus()

	for _, chatID := range []int64{1, 2} {
		messages := sender.SentTo(chatID)
		if len(messages) != 1 {
			t.Fatalf("expected one message for chat %d, got %v", chatID, messages)
		}
		if messages[0] != "Світла немає." {
			t.Fatalf("unexpected message %q for chat %d", messages[0], chatID)
		}
	}

	if bot.currentGridState != 0 || bot.previousGridState != 0 {
		t.Fatalf("expected current and previous grid state to be 0, got current=%d previous=%d", bot.currentGridState, bot.previousGridState)
	}
}

func TestProcessOutageNotificationDebounce(t *testing.T) {
	sender := newFakeSender()
	bot := &Bot{
		chatIDs:          map[int64]bool{1: true},
		sendMessage:      sender.Send,
		lastNotification: make(map[notificationType]time.Time),
	}

	if !bot.processOutageNotification() {
		t.Fatal("expected first outage notification to be sent")
	}

	if messages := sender.SentTo(1); len(messages) != 1 || messages[0] != outageMessage {
		t.Fatalf("unexpected outage message payloads: %v", messages)
	}

	if bot.processOutageNotification() {
		t.Fatal("expected second outage notification to be debounced")
	}

	if messages := sender.SentTo(1); len(messages) != 1 {
		t.Fatalf("expected single outage notification after debounce, got %v", messages)
	}

	bot.mu.Lock()
	bot.lastNotification[notificationOutage] = time.Now().Add(-3 * time.Minute)
	bot.mu.Unlock()

	if !bot.processOutageNotification() {
		t.Fatal("expected outage notification after cooldown")
	}

	if messages := sender.SentTo(1); len(messages) != 2 {
		t.Fatalf("expected two outage notifications after cooldown, got %v", messages)
	}
}

func TestHandleStatusCommandResponds(t *testing.T) {
	sender := newFakeSender()
	bot := &Bot{
		chatIDs:          make(map[int64]bool),
		sendMessage:      sender.Send,
		lastNotification: make(map[notificationType]time.Time),
	}
	bot.currentGridState = 0

	bot.handleStatusCommand(42)

	messages := sender.SentTo(42)
	if len(messages) != 1 {
		t.Fatalf("expected status response, got %v", messages)
	}
	if messages[0] != "Світла немає." {
		t.Fatalf("unexpected status payload %q", messages[0])
	}
}
