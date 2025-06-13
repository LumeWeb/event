package event

import (
	"reflect"
	"sort"
)

// Listener interface
type Listener[T any] interface {
	Handle(e Event[T]) error
}

// ListenerFunc func definition.
type ListenerFunc[T any] func(e Event[T]) error

// Handle event. implements the Listener interface
func (fn ListenerFunc[T]) Handle(e Event[T]) error {
	return fn(e)
}

// ListenerItem storage a event listener and it's priority value.
type ListenerItem[T any] struct {
	Priority int
	Listener Listener[T]
}

/*************************************************************
 * Listener Queue
 *************************************************************/

// ListenerQueue storage sorted Listener instance.
type ListenerQueue[T any] struct {
	items []*ListenerItem[T]
}

// Len get items length
func (lq *ListenerQueue[T]) Len() int {
	return len(lq.items)
}

// IsEmpty get items length == 0
func (lq *ListenerQueue[T]) IsEmpty() bool {
	return len(lq.items) == 0
}

// Push get items length
func (lq *ListenerQueue[T]) Push(li *ListenerItem[T]) *ListenerQueue[T] {
	lq.items = append(lq.items, li)
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
	if listener == nil {
		return
	}

	compareKey := getListenCompareKey(listener)

	var newItems []*ListenerItem[T]
	for _, li := range lq.items {
		liKey := getListenCompareKey(li.Listener)

		if reflect.DeepEqual(liKey, compareKey) {
			continue // skip this listener (remove it)
		}
		newItems = append(newItems, li)
	}

	lq.items = newItems
}

// Clear all listeners
func (lq *ListenerQueue[T]) Clear() {
	lq.items = lq.items[:0]
}

// getListenCompareKey get listener compare key
func getListenCompareKey[T any](src Listener[T]) any {
	val := reflect.ValueOf(src)

	// If it's a pointer to a struct, dereference it for comparison
	if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Struct {
		return val.Elem().Interface()
	}

	return val
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
