package proxy

import (
	"testing"
)

func TestProxyPool(t *testing.T) {
	proxies := []string{"p1", "p2", "p3"}
	pool := NewProxyPool(proxies)

	// Test rotation
	if p := pool.GetNext(); p != "p1" {
		t.Errorf("Expected p1, got %s", p)
	}
	if p := pool.GetNext(); p != "p2" {
		t.Errorf("Expected p2, got %s", p)
	}
	if p := pool.GetNext(); p != "p3" {
		t.Errorf("Expected p3, got %s", p)
	}
	if p := pool.GetNext(); p != "p1" {
		t.Errorf("Expected p1, got %s", p)
	}

	// Test failure
	pool.MarkFailed("p2")

	// Should skip p2
	// Current index is at p2 (after returning p1)
	if p := pool.GetNext(); p != "p3" {
		t.Errorf("Expected p3 (skipping p2), got %s", p)
	}

	// Next should be p1
	if p := pool.GetNext(); p != "p1" {
		t.Errorf("Expected p1, got %s", p)
	}

	// Next should be p3 (skipping p2)
	if p := pool.GetNext(); p != "p3" {
		t.Errorf("Expected p3, got %s", p)
	}

	// Mark healthy
	pool.MarkHealthy("p2")

	// Should include p2 again
	// Current index is at p1 (after returning p3)
	if p := pool.GetNext(); p != "p1" {
		t.Errorf("Expected p1, got %s", p)
	}
	if p := pool.GetNext(); p != "p2" {
		t.Errorf("Expected p2, got %s", p)
	}
}
