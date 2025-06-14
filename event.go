// Package event is lightweight event manager and dispatcher implements by Go.
package event

import "fmt"

// wildcard event name
const (
	Wildcard = "*"
	AnyNode  = "*"
	AllNode  = "**"
)

const (
	// ModeSimple old mode, simple match group listener.
	//
	//  - "*" only allow one and must at end
	//
	// Support: "user.*" -> match "user.created" "user.updated"
	ModeSimple uint8 = iota

	// ModePath path mode.
	//
	//  - "*" matches any sequence of non . characters (like at path.Match())
	//  - "**" match all characters to end, only allow at start or end on pattern.
	//
	// Support like this:
	// 	"eve.some.*.*"       -> match "eve.some.thing.run" "eve.some.thing.do"
	// 	"eve.some.*.run"     -> match "eve.some.thing.run", but not match "eve.some.thing.do"
	// 	"eve.some.**"        -> match any start with "eve.some.". eg: "eve.some.thing.run" "eve.some.thing.do"
	// 	"**.thing.run"       -> match any ends with ".thing.run". eg: "eve.some.thing.run"
	ModePath
)

// M is short name for map[string]...
type M = map[string]any

// Subscriber interface for event subscribers
type Subscriber[T any] interface {
	SubscribedEvents() map[string]any // map[name]Listener|ListenerItem
}

// EventDispatcher defines core event operations
type EventDispatcher[T any] interface {
	// On registers an event listener with optional priority (higher runs first)
	On(name string, listener Listener[T], priority ...int)
	// Listen is an alias for On()
	Listen(name string, listener Listener[T], priority ...int)
	// Once registers a listener that will only be triggered once
	Once(name string, listener Listener[T], priority ...int)
	// Fire triggers an event by name and returns the event and any error
	Fire(name string, data T) (error, Event[T])
	// Trigger is an alias for Fire()
	Trigger(name string, data T) (error, Event[T])
	// MustFire triggers an event and panics on error
	MustFire(name string, data T) Event[T]
	// MustTrigger is an alias for MustFire()
	MustTrigger(name string, data T) Event[T]
}

// EventBatchManager handles batch event operations
type EventBatchManager[T any] interface {
	// FireBatch triggers multiple events at once and returns all errors
	FireBatch(events ...any) []error
	// FireEvent triggers a pre-constructed event instance
	FireEvent(e Event[T]) error
	// TriggerEvent is an alias for FireEvent()
	TriggerEvent(e Event[T]) error
}

// AsyncEventManager handles asynchronous event operations
type AsyncEventManager[T any] interface {
	// Async fires an event asynchronously via channel
	Async(name string, data T)
	// FireC is an alias for Async()
	FireC(name string, data T)
	// FireAsync fires a pre-constructed event asynchronously via channel
	FireAsync(e Event[T])
	// AsyncFire fires an event asynchronously using goroutine
	AsyncFire(e Event[T])
	// AwaitFire fires an event asynchronously but waits for completion
	AwaitFire(e Event[T]) error
	// CloseWait closes the async channel and waits for pending events
	CloseWait() error
	// Close closes the async channel without waiting
	Close() error
}

// ListenerManager manages event listeners
type ListenerManager[T any] interface {
	// AddListener registers an event listener (alias of On())
	AddListener(name string, listener Listener[T], priority ...int)
	// RemoveListener removes a specific listener by name
	RemoveListener(name string, listener Listener[T])
	// RemoveListeners removes all listeners for an event
	RemoveListeners(name string)
	// HasListeners checks if an event has any listeners
	HasListeners(name string) bool
	// ListenersCount returns the number of listeners for an event
	ListenersCount(name string) int
	// Subscribe registers multiple listeners from a subscriber
	Subscribe(sbr Subscriber[T])
}

// EventDefinitionManager manages event definitions
type EventDefinitionManager[T any] interface {
	// AddEvent registers a pre-defined event instance
	AddEvent(e Event[T])
	// AddEventFc registers an event factory function
	AddEventFc(name string, fc FactoryFunc[T])
	// GetEvent retrieves a pre-defined event by name
	GetEvent(name string) (Event[T], bool)
	// HasEvent checks if a pre-defined event exists
	HasEvent(name string) bool
	// RemoveEvent removes a pre-defined event by name
	RemoveEvent(name string)
	// RemoveEvents removes all pre-defined events
	RemoveEvents()
}

// EventManager combines all event management interfaces
type EventManager[T any] interface {
	EventDispatcher[T]
	EventBatchManager[T]
	AsyncEventManager[T]
	ListenerManager[T]
	EventDefinitionManager[T]
	// Reset clears all listeners and events
	Reset()
}

// Options event manager config options
type Options struct {
	// EnableLock enable lock on fire event. default is False.
	EnableLock bool
	// ChannelSize for fire events by goroutine
	ChannelSize int
	ConsumerNum int
	// MatchMode event name match mode. default is ModeSimple
	MatchMode uint8
}

// OptionFn event manager config option func
type OptionFn func(o *Options)

// UsePathMode set event name match mode to ModePath
func UsePathMode(o *Options) {
	o.MatchMode = ModePath
}

// EnableLock enable lock on fire event.
func EnableLock(enable bool) OptionFn {
	return func(o *Options) {
		o.EnableLock = enable
	}
}

// Event interface
type Event[T any] interface {
	Name() string
	Data() T
	SetData(T) (Event[T], error)
	Abort(bool)
	IsAborted() bool

	// Get a property value by key
	Get(key string) any
	// Set a property value by key
	Set(key string, val any) Event[T]
}

// Cloneable interface. event can be cloned.
type Cloneable[T any] interface {
	Event[T]
	Clone() Event[T]
}

// FactoryFunc for create event instance.
type FactoryFunc[T any] func() Event[T]

// BasicEvent a built-in implements Event interface
type BasicEvent[T any] struct {
	// event name
	name string
	// user data - stores either the typed data or properties map
	data T
	// target
	target any
	// mark is aborted
	aborted bool
}

// New create an event instance
func New[T any](name string, data T) *BasicEvent[T] {
	return NewBasic(name, data)
}

// NewBasic new a basic event instance
func NewBasic[T any](name string, data T) *BasicEvent[T] {
	return &BasicEvent[T]{
		name: name,
		data: data,
	}
}

// Abort event loop exec
func (e *BasicEvent[T]) Abort(abort bool) {
	e.aborted = abort
}

// Fill event data
func (e *BasicEvent[T]) Fill(target any, data T) *BasicEvent[T] {
	e.data = data
	e.target = target
	return e
}

// basicEventAdapter adapts an Event[any] to appear as an Event[T] for a specific listener.
// It also helps in propagating changes (like SetData, Abort) back to the original Event[any].
type basicEventAdapter[T any] struct {
	originalEvent Event[any] // The Event[any] that is being processed by std manager
	typedData     T          // Data of originalEvent, already cast to T
}

func (bea *basicEventAdapter[T]) Name() string { return bea.originalEvent.Name() }
func (bea *basicEventAdapter[T]) Data() T      { return bea.typedData }
func (bea *basicEventAdapter[T]) SetData(d T) (Event[T], error) {
	bea.typedData = d
	_, err := bea.originalEvent.SetData(d) // d (type T) is assigned to `any` field in originalEvent
	return bea, err
}
func (bea *basicEventAdapter[T]) Abort(val bool)  { bea.originalEvent.Abort(val) }
func (bea *basicEventAdapter[T]) IsAborted() bool { return bea.originalEvent.IsAborted() }

// Get a property value by key from original event
func (bea *basicEventAdapter[T]) Get(key string) any {
	switch orig := bea.originalEvent.(type) {
	case *EventTToAnyAdapter[T]:
		return orig.OriginalEvent.Get(key)
	case *BasicEvent[any]:
		return orig.Get(key)
	default:
		return nil
	}
}

// Set a property value by key on original event
func (bea *basicEventAdapter[T]) Set(key string, val any) Event[T] {
	switch orig := bea.originalEvent.(type) {
	case *EventTToAnyAdapter[T]:
		orig.OriginalEvent.Set(key, val)
	case *BasicEvent[any]:
		orig.Set(key, val)
	}
	return bea
}

// newBasicEventAdapter creates a new basicEventAdapter instance
func newBasicEventAdapter[T any](original Event[any], data T) *basicEventAdapter[T] {
	return &basicEventAdapter[T]{
		originalEvent: original,
		typedData:     data,
	}
}

// EventTToAnyAdapter adapts an Event[T] to appear as an Event[any].
// It's used when passing a generic Event[T] to a system expecting Event[any].
type EventTToAnyAdapter[T any] struct {
	OriginalEvent Event[T]
}

// NewEventTToAnyAdapter creates a new adapter for converting Event[T] to Event[any]
func NewEventTToAnyAdapter[T any](original Event[T]) *EventTToAnyAdapter[T] {
	return &EventTToAnyAdapter[T]{
		OriginalEvent: original,
	}
}

func (eta *EventTToAnyAdapter[T]) Name() string { return eta.OriginalEvent.Name() }
func (eta *EventTToAnyAdapter[T]) Data() any    { return eta.OriginalEvent.Data() } // Event[T].Data() returns T, assignable to any
func (eta *EventTToAnyAdapter[T]) SetData(d any) (Event[any], error) {
	dataAsT, ok := d.(T)
	if !ok {
		return eta, fmt.Errorf("event: type error in SetData, event %s. Expected data type %T, got %T", eta.OriginalEvent.Name(), *new(T), d)
	}
	_, err := eta.OriginalEvent.SetData(dataAsT)
	return eta, err
}
func (eta *EventTToAnyAdapter[T]) Abort(val bool)  { eta.OriginalEvent.Abort(val) }
func (eta *EventTToAnyAdapter[T]) IsAborted() bool { return eta.OriginalEvent.IsAborted() }

// Get a property value by key from original event
func (eta *EventTToAnyAdapter[T]) Get(key string) any {
	return eta.OriginalEvent.Get(key)
}

// Set a property value by key on original event
func (eta *EventTToAnyAdapter[T]) Set(key string, val any) Event[any] {
	eta.OriginalEvent.Set(key, val)
	return eta
}

// AttachTo add current event to the event manager.
func (e *BasicEvent[T]) AttachTo(em EventManager[T]) {
	em.AddEvent(e)
}

// Name get event name
func (e *BasicEvent[T]) Name() string {
	return e.name
}

// Data get all data
func (e *BasicEvent[T]) Data() T {
	return e.data
}

// IsAborted check.
func (e *BasicEvent[T]) IsAborted() bool {
	return e.aborted
}

// Clone new instance
func (e *BasicEvent[T]) Clone() Event[T] {
	cp := *e
	return &cp
}

// Target get target
func (e *BasicEvent[T]) Target() any {
	return e.target
}

// SetName set event name
func (e *BasicEvent[T]) SetName(name string) *BasicEvent[T] {
	e.name = name
	return e
}

// SetData set data to the event
func (e *BasicEvent[T]) SetData(data T) (Event[T], error) {
	e.data = data
	return e, nil
}

// SetTarget set event target
func (e *BasicEvent[T]) SetTarget(target any) *BasicEvent[T] {
	e.target = target
	return e
}

// Get a property value by key
func (e *BasicEvent[T]) Get(key string) any {
	if m, ok := any(e.data).(map[string]any); ok {
		return m[key]
	}
	return nil
}

// Set a property value by key
func (e *BasicEvent[T]) Set(key string, val any) Event[T] {
	if m, ok := any(e.data).(map[string]any); ok {
		m[key] = val
	}
	return e
}
