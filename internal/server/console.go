package server

import "sync"

const consoleBufferSize = 500

// Console stores recent lines and fans out live output to subscribers.
type Console struct {
	mu          sync.RWMutex
	lines       []string
	subscribers map[chan string]struct{}
}

// NewConsole creates a console broadcaster.
func NewConsole() *Console {
	return &Console{subscribers: map[chan string]struct{}{}}
}

// Append stores a line and broadcasts it to subscribers.
func (c *Console) Append(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.lines = append(c.lines, line)
	if len(c.lines) > consoleBufferSize {
		c.lines = c.lines[len(c.lines)-consoleBufferSize:]
	}

	for ch := range c.subscribers {
		select {
		case ch <- line:
		default:
		}
	}
}

// History returns a copy of console history.
func (c *Console) History() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, len(c.lines))
	copy(out, c.lines)
	return out
}

// Subscribe creates a line stream channel.
func (c *Console) Subscribe() chan string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ch := make(chan string, 100)
	c.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes and closes a subscription channel.
func (c *Console) Unsubscribe(ch chan string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, ch)
	close(ch)
}
