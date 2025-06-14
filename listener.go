package event

import (
	"fmt"
	"sort"
	"sync/atomic"
)

// Listener interface
type Listener[T any] interface {
	Handle(e Event[T]) error
	ID() string // Returns a unique identifier for the listener
}

var _ Listener[any] = (*ListenerFunc[any])(nil) // Ensure ListenerFunc implements Listener

// EventHandlerFunc defines the function signature for handling events
type EventHandlerFunc[T any] func(e Event[T]) error

// ListenerFunc wraps a function with an ID to implement Listener
type ListenerFunc[T any] struct {
	fn EventHandlerFunc[T]
	id string
}

var listenerCounter uint64

// NewListenerFunc creates a new ListenerFunc with auto-generated ID
func NewListenerFunc[T any](fn EventHandlerFunc[T]) Listener[T] {
	if fn == nil {
		panic("event: listener function cannot be nil")
	}
	counter := atomic.AddUint64(&listenerCounter, 1)
	return &ListenerFunc[T]{
		fn: fn,
		id: fmt.Sprintf("func_%d", counter),
	}
}

// Handle implements Listener interface
func (lf *ListenerFunc[T]) Handle(e Event[T]) error {
	return lf.fn(e)
}

// ID returns the listener's unique identifier
func (lf *ListenerFunc[T]) ID() string {
	return lf.id
}

// ListenerItem storage a event listener and it's priority value.
type ListenerItem[T any] struct {
	Priority int
	Listener Listener[T]
	id       string // Stores the listener's unique ID
}

// NewListenerItem creates a new ListenerItem instance
func NewListenerItem[T any](listener Listener[T], priority int) *ListenerItem[T] {
	if listener == nil {
		panic("event: listener cannot be nil")
	}
	return &ListenerItem[T]{
		Priority: priority,
		Listener: listener,
		id:       listener.ID(),
	}
}

/*************************************************************
 * Listener Queue
 *************************************************************/

// ListenerQueue storage sorted Listener instance.
type ListenerQueue[T any] struct {
	items []*ListenerItem[T]
	index map[string]*ListenerItem[T] // for O(1) lookups by ID
}

// Len get items length
func (lq *ListenerQueue[T]) Len() int {
	return len(lq.items)
}

// IsEmpty get items length == 0
func (lq *ListenerQueue[T]) IsEmpty() bool {
	return len(lq.items) == 0
}

// Push add listener item to queue
func (lq *ListenerQueue[T]) Push(li *ListenerItem[T]) *ListenerQueue[T] {
	if lq.index == nil {
		lq.index = make(map[string]*ListenerItem[T])
	}

	// Only add if not already present
	if _, exists := lq.index[li.id]; !exists {
		lq.items = append(lq.items, li)
		lq.index[li.id] = li
	}
	return lq
}

// Sort the queue items by ListenerItem's priority.
//
// Priority:
//
//	High > Low
func (lq *ListenerQueue[T]) Sort() *ListenerQueue[T] {
	// if lq.IsEmpty() {
	// 	return lq
	// }
	ls := ByPriorityItems[T](lq.items)

	// check items is sorted
	if !sort.IsSorted(ls) {
		sort.Sort(ls)
	}

	return lq
}

// Items get all ListenerItem
func (lq *ListenerQueue[T]) Items() []*ListenerItem[T] {
	return lq.items
}

// Remove a listener from the queue
func (lq *ListenerQueue[T]) Remove(listener Listener[T]) {
	if listener == nil || lq.index == nil {
		return
	}

	targetID := listener.ID()

	// Remove from index first
	delete(lq.index, targetID)

	// Rebuild items slice without the removed listener
	newItems := make([]*ListenerItem[T], 0, len(lq.items)-1)
	for _, li := range lq.items {
		if li.Listener != nil && li.id != targetID {
			newItems = append(newItems, li)
		}
	}
	lq.items = newItems
}

// Clear all listeners
func (lq *ListenerQueue[T]) Clear() {
	lq.items = lq.items[:0]
	if lq.index != nil {
		lq.index = make(map[string]*ListenerItem[T])
	}
}

/*************************************************************
 * Sorted PriorityItems
 *************************************************************/

// ByPriorityItems type. implements the sort.Interface
type ByPriorityItems[T any] []*ListenerItem[T]

// Len get items length
func (ls ByPriorityItems[T]) Len() int {
	return len(ls)
}

// Less implements the sort.Interface.Less.
func (ls ByPriorityItems[T]) Less(i, j int) bool {
	return ls[i].Priority > ls[j].Priority
}

// Swap implements the sort.Interface.Swap.
func (ls ByPriorityItems[T]) Swap(i, j int) {
	ls[i], ls[j] = ls[j], ls[i]
}
