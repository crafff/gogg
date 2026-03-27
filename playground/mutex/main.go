package main

import (
	"fmt"
	"sync"
	"time"
)

// SafeCounter is safe to use concurrently.
type SafeCounter struct {
	mu sync.Mutex
	v  map[string]int
}

// Inc increments the counter for the given key.
func (c *SafeCounter) Inc(key string) {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.v[key]++
	c.mu.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) Value(key string) int {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.v[key]
}

func main() {
	var wg sync.WaitGroup
	start := time.Now()
	c := SafeCounter{v: make(map[string]int)}
	for i := 0; i < 1000; i++ {
		wg.Go(func() {
			c.Inc("somekey")
		})
		// c.Inc("somekey")
	}
	wg.Wait()
	elapsed := time.Since(start)
	// time.Sleep(time.Second).Round(time.Millisecond)
	fmt.Printf("[%6s]\n", elapsed)
	fmt.Println(c.Value("somekey"))
}
