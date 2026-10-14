package events

import "testing"

func TestEverySubscriberGetsEveryEventInOrder(t *testing.T) {
	b := New()
	a, cancelA := b.Subscribe()
	c, cancelC := b.Subscribe()
	defer cancelA()
	defer cancelC()
	for _, typ := range []string{"one", "two"} {
		b.Publish(Event{Type: typ})
	}
	for _, ch := range []<-chan Event{a, c} {
		if e := <-ch; e.Type != "one" {
			t.Errorf("first %q", e.Type)
		}
		if e := <-ch; e.Type != "two" {
			t.Errorf("second %q", e.Type)
		}
	}
}

func TestASlowSubscriberIsDroppedNotWaitedFor(t *testing.T) {
	b := New()
	slow, cancel := b.Subscribe()
	defer cancel()
	fast, cancelFast := b.Subscribe()
	defer cancelFast()
	for i := 0; i < buffer+1; i++ {
		b.Publish(Event{Type: "x"}) // must not block
		<-fast
	}
	n := 0
	for range slow {
		n++
	}
	if n != buffer {
		t.Errorf("slow subscriber got %d before being dropped, want %d", n, buffer)
	}
}

func TestCancelEndsTheSubscription(t *testing.T) {
	b := New()
	ch, cancel := b.Subscribe()
	cancel()
	cancel() // twice is fine
	b.Publish(Event{Type: "after"})
	if _, ok := <-ch; ok {
		t.Error("a cancelled subscription received an event")
	}
}
