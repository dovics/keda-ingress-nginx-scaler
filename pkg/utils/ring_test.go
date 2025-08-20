package utils

import (
	"testing"
)

func TestNewRing(t *testing.T) {
	ring := NewRing[int](5)
	if ring == nil {
		t.Error("Expected ring to be non-nil")
	}

	if ring.Count() != 0 {
		t.Errorf("Expected count to be 0, got %d", ring.Count())
	}
}

func TestRingEnqueue(t *testing.T) {
	ring := NewRing[int](3)

	// Test adding items
	ring.Enqueue(1)
	if ring.Count() != 1 {
		t.Errorf("Expected count to be 1, got %d", ring.Count())
	}
	if ring.Get(0) != 1 {
		t.Errorf("Expected Get(0) to be 1, got %d", ring.Get(0))
	}

	ring.Enqueue(2)
	if ring.Count() != 2 {
		t.Errorf("Expected count to be 2, got %d", ring.Count())
	}
	if ring.Get(1) != 2 {
		t.Errorf("Expected Get(1) to be 2, got %d", ring.Get(1))
	}

	ring.Enqueue(3)
	if ring.Count() != 3 {
		t.Errorf("Expected count to be 3, got %d", ring.Count())
	}
	if ring.Get(2) != 3 {
		t.Errorf("Expected Get(2) to be 3, got %d", ring.Get(2))
	}
}

func TestRingGetLatest(t *testing.T) {
	ring := NewRing[int](3)

	// Test with one item
	ring.Enqueue(10)
	if ring.GetLatest() != 10 {
		t.Errorf("Expected latest to be 10, got %d", ring.GetLatest())
	}

	// Test with multiple items
	ring.Enqueue(20)
	if ring.GetLatest() != 20 {
		t.Errorf("Expected latest to be 20, got %d", ring.GetLatest())
	}

	ring.Enqueue(30)
	if ring.GetLatest() != 30 {
		t.Errorf("Expected latest to be 30, got %d", ring.GetLatest())
	}
}

func TestRingCircularBehavior(t *testing.T) {
	ring := NewRing[int](3)

	// Fill the ring
	ring.Enqueue(1)
	ring.Enqueue(2)
	ring.Enqueue(3)

	// Check initial state
	if ring.Count() != 3 {
		t.Errorf("Expected count to be 3, got %d", ring.Count())
	}
	if ring.Get(0) != 1 {
		t.Errorf("Expected Get(0) to be 1, got %d", ring.Get(0))
	}
	if ring.Get(1) != 2 {
		t.Errorf("Expected Get(1) to be 2, got %d", ring.Get(1))
	}
	if ring.Get(2) != 3 {
		t.Errorf("Expected Get(2) to be 3, got %d", ring.Get(2))
	}
	if ring.GetLatest() != 3 {
		t.Errorf("Expected latest to be 3, got %d", ring.GetLatest())
	}

	// Add more items to test circular behavior
	ring.Enqueue(4)        // Should overwrite index 0
	if ring.Count() != 4 { // Count keeps increasing
		t.Errorf("Expected count to be 4, got %d", ring.Count())
	}
	if ring.Get(0) != 4 { // 1 was overwritten by 4
		t.Errorf("Expected Get(0) to be 4, got %d", ring.Get(0))
	}
	if ring.Get(1) != 2 {
		t.Errorf("Expected Get(1) to be 2, got %d", ring.Get(1))
	}
	if ring.Get(2) != 3 {
		t.Errorf("Expected Get(2) to be 3, got %d", ring.Get(2))
	}
	if ring.GetLatest() != 4 { // Latest should be 4
		t.Errorf("Expected latest to be 4, got %d", ring.GetLatest())
	}

	ring.Enqueue(5)       // Should overwrite index 1
	if ring.Get(1) != 5 { // 2 was overwritten by 5
		t.Errorf("Expected Get(1) to be 5, got %d", ring.Get(1))
	}
	if ring.GetLatest() != 5 { // Latest should be 5
		t.Errorf("Expected latest to be 5, got %d", ring.GetLatest())
	}
}

func TestRingGetBefore(t *testing.T) {
	ring := NewRing[int](4)

	// Fill the ring
	ring.Enqueue(10)
	ring.Enqueue(20)
	ring.Enqueue(30)
	ring.Enqueue(40)

	// Test GetBefore function
	if ring.GetBefore(0) != 40 { // Latest item
		t.Errorf("Expected GetBefore(0) to be 40, got %d", ring.GetBefore(0))
	}
	if ring.GetBefore(1) != 30 { // One before latest
		t.Errorf("Expected GetBefore(1) to be 30, got %d", ring.GetBefore(1))
	}
	if ring.GetBefore(2) != 20 { // Two before latest
		t.Errorf("Expected GetBefore(2) to be 20, got %d", ring.GetBefore(2))
	}
	if ring.GetBefore(3) != 10 { // Three before latest
		t.Errorf("Expected GetBefore(3) to be 10, got %d", ring.GetBefore(3))
	}

	// Test circular behavior with GetBefore
	ring.Enqueue(50)             // Overwrites 10 at index 0
	if ring.GetBefore(0) != 50 { // Latest item
		t.Errorf("Expected GetBefore(0) to be 50, got %d", ring.GetBefore(0))
	}
	if ring.GetBefore(1) != 40 { // One before latest
		t.Errorf("Expected GetBefore(1) to be 40, got %d", ring.GetBefore(1))
	}
	if ring.GetBefore(2) != 30 { // Two before latest
		t.Errorf("Expected GetBefore(2) to be 30, got %d", ring.GetBefore(2))
	}
	if ring.GetBefore(3) != 20 { // Three before latest
		t.Errorf("Expected GetBefore(3) to be 20, got %d", ring.GetBefore(3))
	}

	// Test circular behavior with GetBefore
	ring.Enqueue(60)             // Overwrites 10 at index 0
	if ring.GetBefore(0) != 60 { // Latest item
		t.Errorf("Expected GetBefore(0) to be 50, got %d", ring.GetBefore(0))
	}
	if ring.GetBefore(1) != 50 { // One before latest
		t.Errorf("Expected GetBefore(1) to be 40, got %d", ring.GetBefore(1))
	}
	if ring.GetBefore(2) != 40 { // Two before latest
		t.Errorf("Expected GetBefore(2) to be 30, got %d", ring.GetBefore(2))
	}
	if ring.GetBefore(3) != 30 { // Three before latest
		t.Errorf("Expected GetBefore(3) to be 20, got %d", ring.GetBefore(3))
	}
}

func TestRingWithStringType(t *testing.T) {
	ring := NewRing[string](3)

	ring.Enqueue("first")
	if ring.GetLatest() != "first" {
		t.Errorf("Expected latest to be 'first', got '%s'", ring.GetLatest())
	}

	ring.Enqueue("second")
	if ring.GetLatest() != "second" {
		t.Errorf("Expected latest to be 'second', got '%s'", ring.GetLatest())
	}

	ring.Enqueue("third")
	if ring.GetLatest() != "third" {
		t.Errorf("Expected latest to be 'third', got '%s'", ring.GetLatest())
	}

	if ring.Get(0) != "first" {
		t.Errorf("Expected Get(0) to be 'first', got '%s'", ring.Get(0))
	}
	if ring.Get(1) != "second" {
		t.Errorf("Expected Get(1) to be 'second', got '%s'", ring.Get(1))
	}
	if ring.Get(2) != "third" {
		t.Errorf("Expected Get(2) to be 'third', got '%s'", ring.Get(2))
	}
}
