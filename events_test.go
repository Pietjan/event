package events

import (
	"sync"
	"testing"
	"time"
)

// TestProcessorOnAndEmit tests if listeners are called when an event is emitted.
func TestProcessorOnAndEmit(t *testing.T) {
	processor := NewProcessor(4, 10)  // 4 workers, job queue size 10
	defer processor.workerPool.Stop() // Ensure worker pool is stopped

	var wg sync.WaitGroup
	wg.Add(1)

	// Register a listener
	processor.On("test_event", func(e *Event) {
		if e.Name != "test_event" {
			t.Errorf("Expected event name 'test_event', got %s", e.Name)
		}
		if e.Data != "test_data" {
			t.Errorf("Expected event data 'test_data', got %v", e.Data)
		}
		wg.Done()
	})

	// Emit an event
	processor.Emit("test_event", "test_data")

	// Wait for the listener to be called
	if waitTimeout(&wg, 2*time.Second) {
		t.Errorf("Listener was not called in time")
	}
}

// TestListenerCancellation tests if a listener can be successfully cancelled.
func TestListenerCancellation(t *testing.T) {
	processor := NewProcessor(4, 10)
	defer processor.workerPool.Stop()

	var listenerCalled bool
	var mu sync.Mutex

	// Register a listener and get the cancel function
	cancel := processor.On("test_event_cancel", func(e *Event) {
		mu.Lock()
		defer mu.Unlock()
		listenerCalled = true
	})

	// Cancel the listener
	cancel()

	// Emit an event
	processor.Emit("test_event_cancel", "test_data")

	// Wait for a short time to see if the listener is incorrectly called
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if listenerCalled {
		t.Errorf("Listener was called after cancellation")
	}
}

// TestWorkerPoolConcurrency tests if the worker pool processes events concurrently.
func TestWorkerPoolConcurrency(t *testing.T) {
	processor := NewProcessor(2, 10) // 2 workers
	defer processor.workerPool.Stop()

	var mu sync.Mutex
	var processedEvents []string

	// Register listeners
	processor.On("event1", func(e *Event) {
		mu.Lock()
		processedEvents = append(processedEvents, "event1")
		mu.Unlock()
	})
	processor.On("event2", func(e *Event) {
		mu.Lock()
		processedEvents = append(processedEvents, "event2")
		mu.Unlock()
	})

	// Emit multiple events
	processor.Emit("event1", "data1")
	processor.Emit("event2", "data2")

	// Wait for a short time to allow processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(processedEvents) != 2 {
		t.Errorf("Expected 2 events to be processed, got %d", len(processedEvents))
	}
}

// waitTimeout waits for the wait group for the specified timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // Completed normally
	case <-time.After(timeout):
		return true // Timed out
	}
}
