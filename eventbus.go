// Copyright 2026 Anqor LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dispatch

import (
	"fmt"
	"log"
	"sync"
)

// EventBus is the central registry for all named events.
// Events must be declared before they can be subscribed to or emitted.
//
// EventBus is safe for concurrent use. DeclareEvent and Subscribe are
// typically called once during startup; Emit is called from multiple
// goroutines during operation.
type EventBus struct {
	mu          sync.RWMutex
	declared    map[string]bool
	subscribers map[string][]EventFunc
}

// DeclareEvent registers a named event.
// Must be called before Subscribe or Emit for this event name.
// Calling DeclareEvent for an already-declared event is idempotent.
func (eb *EventBus) DeclareEvent(name string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.declared[name] = true
	if _, exists := eb.subscribers[name]; !exists {
		eb.subscribers[name] = nil
	}
}

// Subscribe registers fn as a handler for the named event.
// Returns ErrEventNotFound if the event has not been declared.
func (eb *EventBus) Subscribe(name string, fn EventFunc) error {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if !eb.declared[name] {
		return fmt.Errorf("%w: %q", ErrEventNotFound, name)
	}
	eb.subscribers[name] = append(eb.subscribers[name], fn)
	return nil
}

// Emit dispatches payload to all subscribers of the named event.
//
// ctx is passed as-is to every subscriber. Each subscriber runs in its own
// goroutine (fire-and-forget). Subscriber errors are logged and never
// propagated to the emitter. If the event has not been declared, a warning
// is logged and the call returns.
func (eb *EventBus) Emit(name string, ctx any, payload any) {
	eb.mu.RLock()
	if !eb.declared[name] {
		eb.mu.RUnlock()
		log.Printf("dispatch: Emit called for undeclared event %q; ignoring", name)
		return
	}
	fns := make([]EventFunc, len(eb.subscribers[name]))
	copy(fns, eb.subscribers[name])
	eb.mu.RUnlock()

	for _, fn := range fns {
		fn := fn
		go func() {
			if err := fn(ctx, payload); err != nil {
				log.Printf("dispatch: subscriber error for event %q: %v", name, err)
			}
		}()
	}
}
