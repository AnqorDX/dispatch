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

// EventFunc is the handler type for event subscriptions.
// ctx is whatever the emitter passed; subscribers are responsible for
// asserting the concrete type they expect.
type EventFunc func(ctx any, payload any) error

// NewEventBus creates a new, empty EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		declared:    make(map[string]bool),
		subscribers: make(map[string][]EventFunc),
	}
}
