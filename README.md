# dispatch

`dispatch` is a generic, context-aware event bus for Go.

Events must be declared before they can be subscribed to or emitted. Each bus
carries a user-defined context type (`Ctx`) that flows from the emitter to
every subscriber — letting you stamp correlation IDs, trace spans, or any other
ambient data without coupling the bus to a specific framework.

## API

### Types

#### `ContextFactory[Ctx]`

```go
type ContextFactory[Ctx any] func(emitterCtx Ctx) Ctx
```

Derives a subscriber's context from the emitter's context. Called once per
subscriber per emission, inside the subscriber's goroutine. Must handle the
zero value of `Ctx` (produced when `Emit` is used instead of
`EmitWithContext`).

#### `EventFunc[Ctx]`

```go
type EventFunc[Ctx any] func(ctx Ctx, payload any) error
```

The handler signature for event subscriptions. `ctx` is produced by the bus's
`ContextFactory`; `payload` is whatever was passed to `Emit` /
`EmitWithContext`. Errors returned by the handler are logged and never
propagated to the emitter.

#### `EventBus[Ctx]`

The central registry. Safe for concurrent use after startup.

### Functions & Methods

| Symbol | Description |
|---|---|
| `NewEventBus[Ctx](factory)` | Create a new bus. Pass `nil` for factory to forward the emitter's context unchanged. |
| `(*EventBus[Ctx]).DeclareEvent(name)` | Register a named event. Idempotent. Must be called before `Subscribe` or `EmitWithContext`. |
| `(*EventBus[Ctx]).Subscribe(name, fn)` | Attach a handler. Returns `ErrEventNotFound` if the event was not declared. |
| `(*EventBus[Ctx]).EmitWithContext(name, ctx, payload)` | Dispatch `payload` to all subscribers. Each subscriber runs in its own goroutine (fire-and-forget). Logs a warning for undeclared events. |
| `(*EventBus[Ctx]).Emit(name, payload)` | Shorthand — calls `EmitWithContext` with the zero value of `Ctx`. |

### Errors

| Sentinel | Returned by |
|---|---|
| `ErrEventNotFound` | `Subscribe` when the named event has not been declared. |

## Usage example

```go
package main

import (
	"fmt"
	"sync"

	"github.com/AnqorDX/dispatch"
)

// RequestCtx carries per-request ambient data.
type RequestCtx struct {
	CorrelationID string
}

func main() {
	// Build a factory that stamps a new correlation ID for each subscriber.
	factory := dispatch.ContextFactory[RequestCtx](func(emitter RequestCtx) RequestCtx {
		return RequestCtx{CorrelationID: emitter.CorrelationID + ":sub"}
	})

	bus := dispatch.NewEventBus(factory)

	// Declare events once at startup.
	bus.DeclareEvent("order.placed")

	// Subscribe one or more handlers.
	var wg sync.WaitGroup
	wg.Add(1)

	err := bus.Subscribe("order.placed", func(ctx RequestCtx, payload any) error {
		defer wg.Done()
		fmt.Printf("[%s] order received: %v\n", ctx.CorrelationID, payload)
		return nil
	})
	if err != nil {
		panic(err)
	}

	// Emit from a request handler with its context.
	emitterCtx := RequestCtx{CorrelationID: "req-42"}
	bus.EmitWithContext("order.placed", emitterCtx, map[string]any{"id": 7, "item": "widget"})

	wg.Wait()
	// Output: [req-42:sub] order received: map[id:7 item:widget]
}
```

## Concurrency model

- `DeclareEvent` and `Subscribe` are safe to call concurrently.
- `EmitWithContext` / `Emit` snapshot the subscriber list under a read lock,
  then launch one goroutine per subscriber — fire-and-forget.
- Subscriber errors are logged via the standard `log` package and never returned
  to the emitter.
- Two buses are completely independent; events declared on one are invisible to
  the other.