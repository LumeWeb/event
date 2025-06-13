package event

import (
	"fmt"
	"reflect"
)

// std default event manager
var std = NewManager[any]("default")

// StdManagerAdapter adapts Manager[any] to ManagerFace[T]
type StdManagerAdapter[T any] struct {
	Mgr *Manager[any]
}

func (a *StdManagerAdapter[T]) AddEvent(e Event[T]) {
	a.Mgr.AddEvent(&EventTToAnyAdapter[T]{OriginalEvent: e})
}

func (a *StdManagerAdapter[T]) On(name string, listener Listener[T], priority ...int) {
	On(name, listener, priority...)
}

func (a *StdManagerAdapter[T]) Fire(name string, data T) (error, Event[T]) {
	return Fire(name, data)
}

func (a *StdManagerAdapter[T]) Subscribe(sbr Subscriber[T]) {
	// Create a wrapper subscriber that adapts the SubscribedEvents map
	wrappedSubscriber := &subscriberWrapper[T]{sbr}
	a.Mgr.Subscribe(wrappedSubscriber)
}

// subscriberWrapper adapts a Subscriber[T] to work with Manager[any]
type subscriberWrapper[T any] struct {
	Subscriber[T]
}

// SubscribedEvents adapts the SubscribedEvents map to use Listener[any]
func (sw *subscriberWrapper[T]) SubscribedEvents() map[string]any {
	originalEvents := sw.Subscriber.SubscribedEvents()
	wrappedEvents := make(map[string]any, len(originalEvents))

	for name, listener := range originalEvents {
		// Dereference pointers first
		val := reflect.ValueOf(listener)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
			listener = val.Interface()
		}

		switch lt := listener.(type) {
		case Listener[T]:
			wrappedEvents[name] = ListenerFunc[any](func(e Event[any]) error {
				dataAsT, ok := e.Data().(T)
				if !ok {
					var zeroT T
					return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T", e.Name(), zeroT, e.Data())
				}
				return lt.Handle(newBasicEventAdapter(e, dataAsT))
			})
		case ListenerItem[T]:
			wrappedEvents[name] = ListenerItem[any]{
				Priority: lt.Priority,
				Listener: ListenerFunc[any](func(e Event[any]) error {
					dataAsT, ok := e.Data().(T)
					if !ok {
						var zeroT T
						return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T", e.Name(), zeroT, e.Data())
					}
					return lt.Listener.Handle(newBasicEventAdapter(e, dataAsT))
				}),
			}
		default:
			panicf("event: invalid listener type %T for event '%s'", listener, name)
		}
	}

	return wrappedEvents
}

// Std get default event manager
func Std() *Manager[any] {
	return std
}

// NewMEvent creates a new BasicEvent with event.M data type
func NewMEvent(name string, data M) *BasicEvent[M] {
	return NewBasic(name, data)
}

// StdForType returns a ManagerFace[T] adapter for the default std manager
func StdForType[T any]() ManagerFace[T] {
	return &StdManagerAdapter[T]{Mgr: std}
}

// Config set default event manager options
func Config(fn ...OptionFn) {
	std.WithOptions(fn...)
}

/*************************************************************
 * Listener
 *************************************************************/

// On register a listener to the event. alias of Listen()
func On[T any](name string, listener Listener[T], priority ...int) {
	if listener == nil {
		panicf("event: the event %q listener cannot be empty", name)
	}
	wrappedListener := ListenerFunc[any](func(e Event[any]) error {
		dataAsT, ok := e.Data().(T)
		if !ok {
			var zeroT T
			return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T", e.Name(), zeroT, e.Data())
		}
		return listener.Handle(newBasicEventAdapter(e, dataAsT))
	})
	std.On(name, wrappedListener, priority...)
}

// Once register a listener to the event. trigger once
func Once[T any](name string, listener Listener[T], priority ...int) {
	// The actual listener that will be registered with std.Once
	var onceWrapper ListenerFunc[any]
	onceWrapper = ListenerFunc[any](func(e Event[any]) error {
		// Remove the listener from std manager before handling.
		// std.RemoveListener needs Listener[any].
		std.RemoveListener(name, onceWrapper)

		dataAsT, ok := e.Data().(T)
		if !ok {
			var zeroT T
			return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T", e.Name(), zeroT, e.Data())
		}
		return listener.Handle(newBasicEventAdapter(e, dataAsT))
	})
	std.On(name, onceWrapper, priority...) // std.Once itself uses std.On then removes, so we adapt for std.On
}

// Listen register a listener to the event
func Listen[T any](name string, listener Listener[T], priority ...int) {
	On(name, listener, priority...) // Delegate to On which has the wrapping logic
}

// AddListeners registers multiple listeners at once.
func AddListeners[T any](listeners map[string]Listener[T], priority ...int) {
	wrappedListeners := make(map[string]Listener[any], len(listeners))
	for name, originalListener := range listeners {
		// Capture loop variables for closure
		listenerName := name
		capturedOriginalListener := originalListener

		wrappedListeners[listenerName] = ListenerFunc[any](func(e Event[any]) error {
			dataAsT, ok := e.Data().(T)
			if !ok {
				var zeroT T
				return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T", e.Name(), zeroT, e.Data())
			}
			return capturedOriginalListener.Handle(newBasicEventAdapter(e, dataAsT))
		})
	}
	std.AddListeners(wrappedListeners, priority...)
}

// Subscribe registers a subscriber with the given manager
func Subscribe[T any](mgr *Manager[T], sbr Subscriber[T]) {
	mgr.Subscribe(sbr)
}

// AsyncFire simple async fire event by 'go' keywords
func AsyncFire[T any](e Event[T]) {
	adaptedEventForStd := &EventTToAnyAdapter[T]{OriginalEvent: e}
	std.AsyncFire(adaptedEventForStd)
}

// Async fire event by channel
func Async[T any](name string, data T) {
	std.Async(name, data) // Manager[any].Async takes (string, any), data T is compatible
}

// FireAsync fire event by channel
func FireAsync[T any](e Event[T]) {
	adaptedEventForStd := &EventTToAnyAdapter[T]{OriginalEvent: e}
	std.FireAsync(adaptedEventForStd)
}

// CloseWait close chan and wait for all async events done.
func CloseWait() error {
	return std.CloseWait()
}

// Trigger alias of Fire
func Trigger[T any](name string, data T) (error, Event[T]) {
	return Fire(name, data) // Delegate to Fire which has the wrapping logic
}

// Fire listeners by name.
func Fire[T any](name string, data T) (error, Event[T]) {
	err, evtAny := std.Fire(name, data) // std.Fire returns Event[any]
	if evtAny == nil {
		// This case should ideally not happen if std.Fire always returns an event instance.
		// Based on Manager.Fire, an event is always created.
		return err, nil
	}

	returnedData, ok := evtAny.Data().(T)
	if !ok {
		var zeroT T
		// Check if data is nil and T is a non-nillable type
		if evtAny.Data() == nil {
			rt := reflect.TypeOf(zeroT)
			if rt != nil { // rt is nil if T is `any` or an untyped nil interface
				isNillable := rt.Kind() == reflect.Ptr ||
					rt.Kind() == reflect.Interface ||
					rt.Kind() == reflect.Map ||
					rt.Kind() == reflect.Slice ||
					rt.Kind() == reflect.Chan ||
					rt.Kind() == reflect.Func
				if !isNillable {
					return fmt.Errorf("event: type error for event '%s'. Event data is nil, but listener expected non-nillable type %T. Original error: %w", evtAny.Name(), zeroT, err), nil
				}
			}
			// If data is nil and T is nillable, returnedData (zeroT) is correctly nil/zero for T.
		} else {
			// Data is not nil, but not of type T.
			return fmt.Errorf("event: type error for event '%s'. Expected data type %T, but got %T. Original error: %w", evtAny.Name(), zeroT, evtAny.Data(), err), nil
		}
	}

	return err, newBasicEventAdapter(evtAny, returnedData)
}

// FireEvent fire listeners by Event instance.
func FireEvent[T any](e Event[T]) error {
	adaptedEventForStd := &EventTToAnyAdapter[T]{OriginalEvent: e}
	return std.FireEvent(adaptedEventForStd)
}

// TriggerEvent alias of FireEvent
func TriggerEvent[T any](e Event[T]) error {
	return FireEvent(e) // Delegate to FireEvent which has the wrapping logic
}

// MustFire fire event by name. will panic on error
func MustFire[T any](name string, data T) Event[T] {
	err, evt := Fire(name, data) // Use the wrapped Fire
	if err != nil {
		panic(err)
	}
	return evt
}

// MustTrigger alias of MustFire
func MustTrigger[T any](name string, data T) Event[T] {
	return MustFire(name, data) // Delegate to MustFire
}

// FireBatch fire multi event at once.
func FireBatch(es ...any) []error {
	return std.FireBatch(es...)
}

// HasListeners has listeners for the event name.
func HasListeners(name string) bool {
	return std.HasListeners(name)
}

// Reset the default event manager
func Reset() {
	std.Clear()
}

/*************************************************************
 * Event
 *************************************************************/

// AddEvent add a pre-defined event.
func AddEvent[T any](e Event[T]) {
	adaptedEventForStd := &EventTToAnyAdapter[T]{OriginalEvent: e}
	std.AddEvent(adaptedEventForStd)
}

// AddEventFc add a pre-defined event factory func to manager.
func AddEventFc(name string, fc FactoryFunc[any]) {
	std.AddEventFc(name, fc)
}

// GetEvent get event by name.
func GetEvent(name string) (Event[any], bool) {
	return std.GetEvent(name)
}

// HasEvent has event check.
func HasEvent(name string) bool {
	return std.HasEvent(name)
}

