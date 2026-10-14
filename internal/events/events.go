// Package events is Foreman's in-process publish and subscribe
// (wiki/product/architecture/building-blocks.md#event-bus-events): blocks
// publish what happened, the web server streams it to the browser.
package events

import "sync"

// buffer is how many events a subscriber may lag behind before it is dropped:
// a slow subscriber must never stall a publisher.
const buffer = 256

// Bus delivers every published event to every subscriber.
type Bus struct {
	mu   sync.Mutex
	subs map[chan Event]struct{}
}

// New returns an empty bus.
func New() *Bus { return &Bus{subs: map[chan Event]struct{}{}} }

// Publish never blocks. A subscriber that fell behind is dropped: its channel
// closes, and it subscribes again and refetches what it shows.
func (b *Bus) Publish(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
			delete(b.subs, ch)
			close(ch)
		}
	}
}

// Subscribe returns the events published from now on, and a function that
// ends the subscription (safe to call more than once).
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
	}
}
