// -----------------------------------------------------------------------
// Last Modified: Friday, 7th November 2025
// Modified By: Kiro AI Assistant
// -----------------------------------------------------------------------

package crawler

import (
	"testing"
	"time"

	"github.com/ternarybob/arbor"
)

func TestChromeDPPool_BasicOperations(t *testing.T) {
	// Create a test logger
	logger := arbor.NewLogger()

	// Create pool configuration
	config := ChromeDPPoolConfig{
		MaxInstances:       2,
		UserAgent:          "Test-Agent/1.0",
		Headless:           true,
		DisableGPU:         true,
		NoSandbox:          true,
		JavaScriptWaitTime: 3 * time.Second,
		RequestTimeout:     30 * time.Second,
	}

	// Create new pool
	pool := NewChromeDPPool(config, logger)

	// Test initial state
	if pool.IsInitialized() {
		t.Error("Pool should not be initialized initially")
	}

	// Test initialization
	err := pool.InitBrowserPool(config)
	if err != nil {
		t.Fatalf("Failed to initialize browser pool: %v", err)
	}

	// Test initialized state
	if !pool.IsInitialized() {
		t.Error("Pool should be initialized after InitBrowserPool")
	}

	// Test getting browser instances
	ctx1, release1, err := pool.GetBrowser()
	if err != nil {
		t.Fatalf("Failed to get browser from pool: %v", err)
	}
	if ctx1 == nil {
		t.Error("Browser context should not be nil")
	}
	if release1 == nil {
		t.Error("Release function should not be nil")
	}

	ctx2, release2, err := pool.GetBrowser()
	if err != nil {
		t.Fatalf("Failed to get second browser from pool: %v", err)
	}
	if ctx2 == nil {
		t.Error("Second browser context should not be nil")
	}

	// Test round-robin allocation (should get different contexts)
	if ctx1 == ctx2 {
		t.Error("Round-robin allocation should return different contexts")
	}

	// Test release functions
	release1()
	release2()

	// Test pool stats
	stats := pool.GetPoolStats()
	if stats["max_instances"] != 2 {
		t.Errorf("Expected max_instances=2, got %v", stats["max_instances"])
	}
	if stats["active_instances"] != 2 {
		t.Errorf("Expected active_instances=2, got %v", stats["active_instances"])
	}
	if stats["initialized"] != true {
		t.Errorf("Expected initialized=true, got %v", stats["initialized"])
	}

	// Test shutdown
	err = pool.ShutdownBrowserPool()
	if err != nil {
		t.Fatalf("Failed to shutdown browser pool: %v", err)
	}

	// Test state after shutdown
	if pool.IsInitialized() {
		t.Error("Pool should not be initialized after shutdown")
	}

	// Test getting browser after shutdown should fail
	_, _, err = pool.GetBrowser()
	if err == nil {
		t.Error("Getting browser after shutdown should fail")
	}
}

func TestChromeDPPool_InvalidConfiguration(t *testing.T) {
	logger := arbor.NewLogger()

	// Test invalid max instances
	config := ChromeDPPoolConfig{
		MaxInstances: 0,
		UserAgent:    "Test-Agent/1.0",
		Headless:     true,
	}

	pool := NewChromeDPPool(config, logger)
	err := pool.InitBrowserPool(config)
	if err == nil {
		t.Error("InitBrowserPool should fail with MaxInstances=0")
	}
}

func TestChromeDPPool_DoubleInitialization(t *testing.T) {
	logger := arbor.NewLogger()

	config := ChromeDPPoolConfig{
		MaxInstances: 1,
		UserAgent:    "Test-Agent/1.0",
		Headless:     true,
		DisableGPU:   true,
		NoSandbox:    true,
	}

	pool := NewChromeDPPool(config, logger)

	// First initialization should succeed
	err := pool.InitBrowserPool(config)
	if err != nil {
		t.Fatalf("First initialization should succeed: %v", err)
	}

	// Second initialization should fail
	err = pool.InitBrowserPool(config)
	if err == nil {
		t.Error("Second initialization should fail")
	}

	// Cleanup
	pool.ShutdownBrowserPool()
}
