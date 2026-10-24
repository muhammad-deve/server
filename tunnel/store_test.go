package tunnel

import (
	"sync"
	"testing"
	"time"
)

// Closing an inspector tab while requests flow used to close a channel that
// Add was about to send on, which panicked and killed the CLI.
func TestUnsubscribeWhileAddingDoesNotPanic(t *testing.T) {
	store := newRequestStore(10)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					store.Add(&CapturedRequest{ID: newRequestID()})
				}
			}
		}()
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		ch := store.Subscribe()
		select {
		case <-ch:
		default:
		}
		store.Unsubscribe(ch)
	}
	close(stop)
	wg.Wait()
}

func TestSubscriberReceivesNewRequests(t *testing.T) {
	store := newRequestStore(10)
	ch := store.Subscribe()
	defer store.Unsubscribe(ch)
	store.Add(&CapturedRequest{ID: "req_1"})
	select {
	case got := <-ch:
		if got.ID != "req_1" {
			t.Fatalf("got %q", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("no request delivered")
	}
}
