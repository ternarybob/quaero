package crawler

import (
	"container/heap"
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// URLQueue implements a priority queue with deduplication using Go's heap and sync primitives
type URLQueue struct {
	items  *itemHeap
	seen   map[string]bool // Normalized URL -> seen
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
}

// itemHeap implements heap.Interface for priority ordering
type itemHeap []*URLQueueItem

func (h itemHeap) Len() int { return len(h) }

func (h itemHeap) Less(i, j int) bool {
	// Lower depth first, then priority, then older items first
	if h[i].Depth != h[j].Depth {
		return h[i].Depth < h[j].Depth
	}
	if h[i].Priority != h[j].Priority {
		return h[i].Priority < h[j].Priority
	}
	return h[i].AddedAt.Before(h[j].AddedAt)
}

func (h itemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *itemHeap) Push(x interface{}) {
	*h = append(*h, x.(*URLQueueItem))
}

func (h *itemHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// NewURLQueue creates a new URL queue
func NewURLQueue() *URLQueue {
	h := &itemHeap{}
	heap.Init(h)
	q := &URLQueue{
		items: h,
		seen:  make(map[string]bool),
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push adds a URL to the queue with deduplication check
func (q *URLQueue) Push(item *URLQueueItem) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return false
	}

	normalized := normalizeURL(item.URL)
	if q.seen[normalized] {
		return false // Already seen
	}

	q.seen[normalized] = true
	heap.Push(q.items, item)
	q.cond.Signal() // Wake up one waiting Pop
	return true
}

// Pop removes and returns the highest priority URL (blocking with context support)
// CRITICAL FIX: Uses timeout-based wait instead of goroutines to prevent goroutine leaks
func (q *URLQueue) Pop(ctx context.Context) (*URLQueueItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	const maxWaitTimeout = 10 * time.Second // Maximum wait time before checking context again

	for {
		// Check context cancellation first
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Check if closed
		if q.closed {
			return nil, nil
		}

		// Check if items available
		if q.items.Len() > 0 {
			item := heap.Pop(q.items).(*URLQueueItem)
			return item, nil
		}

		// No items available - wait with timeout to prevent indefinite blocking
		// Set up timeout to broadcast and wake up after maxWaitTimeout
		timer := time.AfterFunc(maxWaitTimeout, func() {
			q.cond.Broadcast()
		})

		// Wait for signal (will be woken by Push(), Close(), or timer)
		q.cond.Wait()

		// Stop timer if it hasn't fired yet
		timer.Stop()

		// Loop will re-check context, closed status, and items
	}
}

// Len returns the number of items in the queue
func (q *URLQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.items.Len()
}

// Close closes the queue and wakes up all waiting Pop calls
func (q *URLQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Contains checks if a URL has been seen (after normalization)
func (q *URLQueue) Contains(rawURL string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	normalized := normalizeURL(rawURL)
	return q.seen[normalized]
}

// Clear removes all items from the queue
func (q *URLQueue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = &itemHeap{}
	heap.Init(q.items)
	q.seen = make(map[string]bool)
}

// normalizeURL canonicalizes URLs for deduplication (strip fragments, sort query params)
func normalizeURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(rawURL))
	}

	// Remove fragment
	u.Fragment = ""

	// Sort query parameters for consistent comparison
	if u.RawQuery != "" {
		query := u.Query()
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		values := url.Values{}
		for _, k := range keys {
			values[k] = query[k]
		}
		u.RawQuery = values.Encode()
	}

	return strings.ToLower(u.String())
}
