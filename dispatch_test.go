package dispatch_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AnqorDX/dispatch"
)

// ---------------------------------------------------------------------------
// DeclareEvent + Subscribe + Emit round-trip
// ---------------------------------------------------------------------------

func TestDeclareSubscribeEmit_RoundTrip(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("user.created")

	var got atomic.Value
	var wg sync.WaitGroup
	wg.Add(1)

	if err := bus.Subscribe("user.created", func(ctx any, payload any) error {
		defer wg.Done()
		got.Store(payload)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe returned unexpected error: %v", err)
	}

	bus.Emit("user.created", "e1", "hello-payload")

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber")
	}

	if got.Load() != "hello-payload" {
		t.Fatalf("expected payload %q, got %v", "hello-payload", got.Load())
	}
}

// ---------------------------------------------------------------------------
// Subscriber receives the exact ctx passed to Emit
// ---------------------------------------------------------------------------

func TestEmit_SubscriberReceivesEmitterCtx(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("ctx.check")

	var gotCtx atomic.Value
	var wg sync.WaitGroup
	wg.Add(1)

	bus.Subscribe("ctx.check", func(ctx any, payload any) error {
		defer wg.Done()
		gotCtx.Store(ctx)
		return nil
	})

	bus.Emit("ctx.check", "emitter-ctx", nil)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscriber")
	}

	received, ok := gotCtx.Load().(string)
	if !ok {
		t.Fatal("subscriber ctx was not stored as a string")
	}
	if received != "emitter-ctx" {
		t.Fatalf("expected ctx %q passed through unchanged, got %q", "emitter-ctx", received)
	}
}

// ---------------------------------------------------------------------------
// Multiple subscribers all called on a single emission
// ---------------------------------------------------------------------------

func TestEmit_MultipleSubscribers_AllCalled(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("multi")

	const n = 5
	var count atomic.Int32
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		bus.Subscribe("multi", func(ctx any, payload any) error {
			defer wg.Done()
			count.Add(1)
			return nil
		})
	}

	bus.Emit("multi", nil, "data")

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for all subscribers")
	}

	if count.Load() != n {
		t.Fatalf("expected %d subscriber calls, got %d", n, count.Load())
	}
}

// ---------------------------------------------------------------------------
// Failing subscriber does not prevent other subscribers from running
// ---------------------------------------------------------------------------

func TestEmit_FailingSubscriber_OthersStillRun(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("independence")

	var secondCalled atomic.Bool
	var wg sync.WaitGroup
	wg.Add(2)

	bus.Subscribe("independence", func(ctx any, payload any) error {
		defer wg.Done()
		return errors.New("subscriber exploded")
	})
	bus.Subscribe("independence", func(ctx any, payload any) error {
		defer wg.Done()
		secondCalled.Store(true)
		return nil
	})

	bus.Emit("independence", nil, "x")

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out — a failing subscriber must not block others")
	}

	if !secondCalled.Load() {
		t.Fatal("second subscriber was not called after first returned an error")
	}
}

// ---------------------------------------------------------------------------
// All subscribers run concurrently
// ---------------------------------------------------------------------------

func TestEmit_SubscribersRunConcurrently(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("concurrent")

	const n = 10
	started := make(chan struct{}, n)
	barrier := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		bus.Subscribe("concurrent", func(ctx any, payload any) error {
			defer wg.Done()
			started <- struct{}{}
			<-barrier
			return nil
		})
	}

	bus.Emit("concurrent", nil, nil)

	for i := 0; i < n; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("timed out after %d/%d goroutines started — subscribers may not be concurrent", i, n)
		}
	}

	close(barrier)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for concurrent subscribers to complete after barrier release")
	}
}

// ---------------------------------------------------------------------------
// Subscribe on undeclared event returns ErrEventNotFound
// ---------------------------------------------------------------------------

func TestSubscribe_UndeclaredEvent_ReturnsErrEventNotFound(t *testing.T) {
	bus := dispatch.NewEventBus()

	err := bus.Subscribe("no.such.event", func(ctx any, payload any) error { return nil })
	if !errors.Is(err, dispatch.ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Emit on undeclared event logs warning and does not panic
// ---------------------------------------------------------------------------

func TestEmit_UndeclaredEvent_DoesNotPanic(t *testing.T) {
	bus := dispatch.NewEventBus()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Emit panicked on undeclared event: %v", r)
		}
	}()

	bus.Emit("never.declared", nil, "payload")
}

// ---------------------------------------------------------------------------
// Two independent buses are independent
// ---------------------------------------------------------------------------

func TestTwoBuses_AreIndependent(t *testing.T) {
	bus1 := dispatch.NewEventBus()
	bus2 := dispatch.NewEventBus()

	bus1.DeclareEvent("shared.name")

	var bus2SubscriberCalled atomic.Bool

	err := bus2.Subscribe("shared.name", func(ctx any, payload any) error {
		bus2SubscriberCalled.Store(true)
		return nil
	})
	if !errors.Is(err, dispatch.ErrEventNotFound) {
		t.Fatalf("bus2.Subscribe should return ErrEventNotFound, got: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	bus1.Subscribe("shared.name", func(ctx any, payload any) error {
		defer wg.Done()
		return nil
	})
	bus1.Emit("shared.name", nil, "bus1-payload")

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for bus1 subscriber")
	}

	if bus2SubscriberCalled.Load() {
		t.Fatal("bus2 subscriber was triggered by a bus1 emission — buses must be independent")
	}
}

// ---------------------------------------------------------------------------
// DeclareEvent is idempotent — a second call must not clear existing subscribers
// ---------------------------------------------------------------------------

func TestDeclareEvent_Idempotent_DoesNotClearSubscribers(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("idempotent")

	var called atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	bus.Subscribe("idempotent", func(ctx any, payload any) error {
		defer wg.Done()
		called.Store(true)
		return nil
	})

	bus.DeclareEvent("idempotent")

	bus.Emit("idempotent", nil, "x")

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out — subscriber may have been lost after second DeclareEvent call")
	}

	if !called.Load() {
		t.Fatal("subscriber was not called after idempotent DeclareEvent")
	}
}

// ---------------------------------------------------------------------------
// Race safety: concurrent DeclareEvent, Subscribe, and Emit
// ---------------------------------------------------------------------------

func TestRaceSafety_ConcurrentOperations(t *testing.T) {
	bus := dispatch.NewEventBus()
	bus.DeclareEvent("race.event")

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			bus.Subscribe("race.event", func(ctx any, payload any) error { return nil })
			bus.Emit("race.event", nil, "data")
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out in race-safety test")
	}
}
