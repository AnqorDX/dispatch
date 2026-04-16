# dispatch

A lightweight, concurrent event bus for Go.

`dispatch` provides a named-event pub/sub system. Events are declared explicitly before use, subscribers register handlers against those names, and emissions fan out to all subscribers concurrently — each in its own goroutine.

## Installation

```
go get github.com/AnqorDX/dispatch
```

## Concepts

**EventBus** — the central registry. Each bus is independent; events declared on one bus have no relation to events on another.

**Declare → Subscribe → Emit** — the only valid order. An event must be declared before it can be subscribed to or emitted. Attempting to subscribe to an undeclared event returns `ErrEventNotFound`. Emitting to an undeclared event logs a warning and does nothing.

**Fire-and-forget** — each subscriber runs in its own goroutine. `Emit` returns immediately without waiting for any subscriber to finish. Subscriber errors are logged and never propagated to the emitter. A failing subscriber does not prevent other subscribers from running.

## Usage

```go
bus := dispatch.NewEventBus()

// Declare events once at startup.
bus.DeclareEvent("order.placed")

// Subscribe one or more handlers.
bus.Subscribe("order.placed", func(ctx any, payload any) error {
    order := payload.(*Order)
    return sendConfirmationEmail(order)
})

bus.Subscribe("order.placed", func(ctx any, payload any) error {
    order := payload.(*Order)
    return updateInventory(order)
})

// Emit from anywhere — both handlers run concurrently.
bus.Emit("order.placed", requestCtx, &Order{ID: "abc123"})
```

## API

### `NewEventBus() *EventBus`
Creates a new, empty event bus.

### `(*EventBus).DeclareEvent(name string)`
Registers a named event. Safe to call multiple times for the same name — subsequent calls are idempotent and do not affect existing subscribers.

### `(*EventBus).Subscribe(name string, fn EventFunc) error`
Registers `fn` as a handler for the named event. Returns `ErrEventNotFound` if the event has not been declared.

### `(*EventBus).Emit(name string, ctx any, payload any)`
Dispatches `payload` to all subscribers of the named event. Each subscriber receives the same `ctx` and `payload` values passed to `Emit`. All subscribers run concurrently in separate goroutines. `Emit` returns before any subscriber completes.

### `type EventFunc func(ctx any, payload any) error`
The handler signature for all subscriptions. A non-nil return value is logged; it does not affect other subscribers or the emitter.

### `var ErrEventNotFound`
Returned by `Subscribe` when the named event has not been declared.

## Guarantees

- **Concurrent-safe.** `DeclareEvent`, `Subscribe`, and `Emit` may be called from multiple goroutines simultaneously.
- **Subscriber isolation.** An error or panic in one subscriber does not affect others.
- **Bus isolation.** Two `EventBus` instances share no state. Events declared on one are invisible to the other.

## License

Apache 2.0. See [LICENSE](LICENSE).
