package main

import "testing"

const inMemoryDSN = "file:testdb?mode=memory&cache=shared"

func TestSubscriberStoreSaveAndList(t *testing.T) {
	store, err := NewSubscriberStore(inMemoryDSN)
	if err != nil {
		t.Fatalf("NewSubscriberStore failed: %v", err)
	}
	defer store.Close()

	ids := []int64{123, 456, 789}
	for _, id := range ids {
		if err := store.Save(id); err != nil {
			t.Fatalf("Save(%d) failed: %v", id, err)
		}
	}

	result, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(result) != len(ids) {
		t.Fatalf("expected %d ids, got %d (%v)", len(ids), len(result), result)
	}

	for i, id := range ids {
		if result[i] != id {
			t.Fatalf("expected id %d at index %d, got %d", id, i, result[i])
		}
	}
}

func TestSubscriberStoreIgnoresDuplicates(t *testing.T) {
	store, err := NewSubscriberStore(inMemoryDSN)
	if err != nil {
		t.Fatalf("NewSubscriberStore failed: %v", err)
	}
	defer store.Close()

	if err := store.Save(777); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := store.Save(777); err != nil {
		t.Fatalf("Save duplicate failed: %v", err)
	}

	result, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(result) != 1 || result[0] != 777 {
		t.Fatalf("expected single value 777, got %v", result)
	}
}

func TestSubscriberStoreRejectsEmptyDSN(t *testing.T) {
	if _, err := NewSubscriberStore(""); err == nil {
		t.Fatal("expected error for empty DSN")
	}
}
