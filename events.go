package events

import (
	"log"
	"sync"
	"time"
)

// Event represents an event with a name, associated data, and a timestamp.
type Event struct {
	Name string
	Data any
	Time time.Time
}

// NewProcessor creates and returns a new Processor instance with a worker pool.
func NewProcessor(workerCount, jobQueueSize int) *Processor {
	wp := NewWorkerPool(workerCount, jobQueueSize)
	wp.Start()
	return &Processor{
		listeners:  make(map[string][]*listener),
		workerPool: wp,
	}
}

// listener holds a handler function that processes an Event.
type listener struct {
	handler func(*Event)
}

// Processor manages event listeners and event dispatching.
type Processor struct {
	listeners  map[string][]*listener
	mutex      sync.RWMutex
	workerPool *WorkerPool
}

// On registers a new event listener for a specific event name.
// Returns a cancel function that can be called to unregister the listener.
func (p *Processor) On(name string, handler func(*Event)) (cancel func()) {
	return p.registerListener(name, handler)
}

// Emit triggers the event with the given name and data, dispatching it to all registered listeners.
func (p *Processor) Emit(name string, data any) {
	e := &Event{
		Name: name,
		Data: data,
		Time: time.Now(),
	}

	go p.dispatch(e) // Dispatch the event asynchronously
}

// Stop stops the worker pool and waits for all workers to finish processing.
func (p *Processor) Stop() {
	p.workerPool.Stop()
}

// dispatch sends the event to all registered listeners for the event's name.
func (p *Processor) dispatch(e *Event) {
	p.mutex.RLock()
	eventListeners, ok := p.listeners[e.Name]
	p.mutex.RUnlock()

	if !ok {
		return // No listeners registered for this event
	}

	// Send each listener's handler as a job to the worker pool
	for _, l := range eventListeners {
		job := Job{
			event:   e,
			handler: l.handler,
		}
		p.workerPool.jobQueue <- job
	}
}

// registerListener adds a listener for the specified event name.
func (p *Processor) registerListener(name string, handler func(*Event)) func() {
	thisListener := &listener{
		handler: handler,
	}

	// Register the listener
	p.mutex.Lock()
	p.listeners[name] = append(p.listeners[name], thisListener)
	p.mutex.Unlock()

	// Return a function to cancel (unregister) the listener
	return func() {
		p.mutex.Lock()
		defer p.mutex.Unlock()

		listeners, ok := p.listeners[name]
		if !ok {
			return // No listeners registered under this name
		}

		// Remove the listener
		for i, l := range listeners {
			if l == thisListener {
				p.listeners[name] = append(listeners[:i], listeners[i+1:]...)
				break
			}
		}

		// If no listeners remain, remove the key from the map
		if len(p.listeners[name]) == 0 {
			delete(p.listeners, name)
		}
	}
}

// Job represents a task to be processed by a worker, consisting of an event and its handler.
type Job struct {
	event   *Event
	handler func(*Event)
}

// WorkerPool manages a pool of workers to process jobs concurrently.
type WorkerPool struct {
	jobQueue    chan Job
	workerCount int
	wg          sync.WaitGroup
}

// NewWorkerPool creates a new WorkerPool with the specified number of workers and job queue size.
func NewWorkerPool(workerCount int, jobQueueSize int) *WorkerPool {
	return &WorkerPool{
		jobQueue:    make(chan Job, jobQueueSize),
		workerCount: workerCount,
	}
}

// Start initializes the worker pool by starting the specified number of workers.
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}
}

// worker is a function run by each worker goroutine, processing jobs from the jobQueue.
func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for job := range wp.jobQueue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("event %s listener handler panic: %v", job.event.Name, r)
				}
			}()
			job.handler(job.event)
		}()
	}
}

// Stop closes the job queue and waits for all workers to finish processing.
func (wp *WorkerPool) Stop() {
	close(wp.jobQueue)
	wp.wg.Wait()
}
