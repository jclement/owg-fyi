// Package counter implements the sacred 90s per-page hit counter.
// Counts live in memory and are debounce-flushed to a JSON file in the
// data dir, so they survive restarts and image updates.
package counter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Counter struct {
	mu     sync.Mutex
	counts map[string]uint64
	path   string
	dirty  bool
}

// New loads (or creates) the counter store and starts the flush loop.
func New(dataDir string) *Counter {
	c := &Counter{
		counts: map[string]uint64{},
		path:   filepath.Join(dataDir, "counters.json"),
	}
	if raw, err := os.ReadFile(c.path); err == nil {
		_ = json.Unmarshal(raw, &c.counts)
	}
	go c.flushLoop()
	return c
}

// Bump increments and returns the count for a page key.
func (c *Counter) Bump(page string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[page]++
	c.dirty = true
	return c.counts[page]
}

func (c *Counter) flushLoop() {
	for range time.Tick(3 * time.Second) {
		c.Flush()
	}
}

// Flush writes the counts atomically if anything changed.
func (c *Counter) Flush() {
	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return
	}
	raw, err := json.MarshalIndent(c.counts, "", " ")
	c.dirty = false
	c.mu.Unlock()
	if err != nil {
		return
	}
	tmp := c.path + ".tmp"
	if os.WriteFile(tmp, raw, 0o644) == nil {
		_ = os.Rename(tmp, c.path)
	}
}
