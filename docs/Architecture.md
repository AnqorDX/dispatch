# dispatch — Architecture

This document describes the internal design of the `dispatch` library: why the pieces are shaped the way they are, how they fit together, and what tradeoffs were made.

---

## The Problem Being Solved

In a plugin-based system, components that do not know about each other need to communicate. The naive solution — direct function calls or shared interfaces — creates import dependencies between packages that should be decoupled. A plugin that handles user authentication should not need to import the package that defines what a "user created" event looks like.

`dispatch` solves this with a name-based event bus: the core declares events with concrete types, and plugins subscribe by name with type-erased handlers. The bus is the only shared dependency.

---

## Package Layout

```
dispatch/
  dispatch.go     — public constructors: NewEventBus, NewEvent[T]
  eventbus.go     — EventBus type: Emit, Subscribe, Unsubscribe
  event.go        — Event[T] type: the typed event implementation
  errors.go       — exported sentinel errors
  internal/
    subscription.go — Subscription: wraps a handler func, exposes Handle
```

The split between `dispatch` and `dispatch/internal` is deliberate:

- `internal` owns the low-level structural types — `Subscription` and `HandlerConfig` — that are returned from or accepted by the public API but whose implementation details should not be part of the public contract.
- The public package owns the generic `Event[T]` and the `EventBus`, which are the types callers actually reason about.

`Event[T]` cannot live in `internal` because Go generics do not allow a non-generic package to store or return a generic type by its concrete parameterization. The type-erasure bridge is the `emitter` interface (see below).

---

## Core Types

### `EventBus`

```
EventBus
  events  map[string]any
```

A flat map from event name to a type-erased value. The map holds `any` because the values are `*Event[T]` for various `T` — Go's type system does not allow a single concrete map value type to express this without losing the type parameter.

The bus exposes three public methods: `Emit`, `Subscribe`, and `Unsubscribe`. All three take a name string, look up the event, cast it to the `emitter` interface, and delegate.

The bus is not safe for concurrent use during event registration (calls to `NewEvent`). Once all events are registered at startup, concurrent `Emit` and `Subscribe` calls are safe because those operations delegate to `Event[T]`, which does not mutate shared bus state.

### `Event[T]`

```
Event[T any]
  name          string
  subscriptions []*internal.Subscription
```

The typed event. `T` is the payload type the core declares at startup. `Event[T]` is never exposed to plugins — they interact with it only through the bus by name.

`Event[T]` implements the unexported `emitter` interface:

```go
type emitter interface {
    emit(payload any) error
    subscribe(handler func(any) error, config ...*internal.HandlerConfig) *internal.Subscription
    unsubscribe(sub *internal.Subscription)
}
```

This interface is the bridge between the type-erased `map[string]any` in `EventBus` and the typed operations on `Event[T]`. The bus stores `any`, retrieves `any`, asserts to `emitter`, and calls the interface methods — never touching the type parameter directly.

### `Subscription`

```
Subscription
  handler  func(any) error
```

A subscription wraps a single handler function. It is created by `Event[T].subscribe` and returned up through the bus to the caller. The caller holds it only if they intend to unsubscribe later; otherwise it can be discarded.

`Subscription` lives in `internal` because its fields should not be directly readable or writable by callers.

---

## The Type-Erasure Bridge

The central design challenge is that `EventBus` needs to store and operate on `*Event[T]` values for arbitrary `T`, but Go does not allow a collection typed as `map[string]*Event[?]`.

The solution is the unexported `emitter` interface:

```
EventBus.events: map[string]any
                         │
                    assert to emitter
                         │
                    Event[T].emit / subscribe / unsubscribe
```

`Event[T]` satisfies `emitter` with its three unexported methods. `EventBus` never needs to know `T` — it just calls the interface. The type assertion from `any` to `emitter` is the single point where the erasure is reversed, and it happens inside `EventBus.getEmitter`, which is the only place in the codebase that performs this cast.

Keeping the `emitter` interface unexported is intentional. It is an implementation detail of how `EventBus` talks to `Event[T]`. External packages should not implement or depend on it.

---

## Emit Semantics

Emission is **fire-and-forget**. Each handler runs in its own goroutine. `Emit` only returns bus-level errors.

```
Emit("user.created", payload)
  → type-assert payload to T            (ErrInvalidEventData if fails)
  → snapshot subscription slice
  → for each subscription:
      go sub.Handle(payload)            (dispatched concurrently, result discarded)
  → return nil                          (does not wait for handlers to complete)
```

**`Emit` returns as soon as all goroutines have been launched.** It does not wait for any handler to complete. Handler execution time, errors, and panics are entirely the handler's concern and have no effect on the bus, the emitter, or any other handler.

**`Emit` only returns errors that belong to the bus.** There are exactly two: `ErrEventNotFound` (the event was never registered) and `ErrInvalidEventData` (the payload cannot be asserted to `T`). Both represent failures of the bus to do its own job. Neither has anything to do with handler behavior.

**Subscribers are independent by definition.** A broken or slow plugin must not prevent other plugins from executing their handlers. Running each handler in its own goroutine is the natural consequence of this — handlers cannot block or fail each other because they do not share an execution context.

**If sequential, ordered, wait-for-completion processing is needed, use the `pipeline` library.** The event bus is a broadcaster. The pipeline library is a coordinator. They are different tools for different jobs.

---

## Type Safety

The bus's type safety guarantee is:

> If `NewEvent[T]` registered the event, then `Emit` will only dispatch to handlers if the payload is a valid `T`.

The assertion `_, ok := payload.(T)` in `Event[T].emit` enforces this. A wrong payload type returns `ErrInvalidEventData` before any handler is called — no handler ever receives a payload it cannot safely use.

Handlers receive `any` rather than `T`. This is the deliberate trade-off for plugin decoupling. The type safety guarantee is one-directional: the bus ensures that what arrives at a handler is always a valid `T`. It is the handler's responsibility to assert the type it needs. A handler that asserts to the wrong type will panic — but this is a programming error in the handler, not a failure of the bus contract.

Note that because `Emit` discards handler return values, the type safety guarantee only flows inward (emitter → handler). There is no bus-mediated type safety on handler outputs — handlers communicate their results through their own mechanisms, not through the bus.

---

## What dispatch Does Not Do

These are intentional non-features, not missing functionality.

**No timeout enforcement.** The bus dispatches and walks away. There is no one home to enforce a timeout. A handler that runs too long is a problem for that handler to solve — the bus has no visibility into it and never will.

**No retry logic.** A handler that fails, fails. That is the handler's problem.

**No correlation IDs or request-response patterns.** The bus is one-directional: core emits, plugins handle. Request-response patterns (where a handler's response is needed by the emitter) are out of scope. That pattern belongs in the pipeline layer.

**No global bus.** Every `EventBus` is independent. There is no package-level singleton. Callers that need a shared bus pass it explicitly.

**No event wildcards or namespacing.** Events are looked up by exact string name. Pattern matching (e.g. subscribing to `"user.*"`) is not supported.