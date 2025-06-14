package event

import (
	"fmt"
	"reflect"
)

// std default event manager
var std = NewManager[any]("default")

// ManagerGetter interface allows getting the underlying *Manager[any]
type ManagerGetter interface {
	GetMgr() *Manager[any]
}

// StdManagerAdapter adapts Manager[any] to EventManager[T]
type StdManagerAdapter[T any] struct {
	Mgr *Manager[any]
	// Uses manager's listener cache
	listenerCache ListenerCache
}

// GetMgr returns the underlying *Manager[any]
func (a *StdManagerAdapter[T]) GetMgr() *Manager[any] {
	return a.Mgr
}

// NewStdManagerAdapter creates a new StdManagerAdapter with initialized listenerCache
func NewStdManagerAdapter[T any](mgr *Manager[any]) *StdManagerAdapter[T] {
	return &StdManagerAdapter[T]{
		Mgr:           mgr,
		listenerCache: mgr.GetListenerCache(),
	}
}

// ensureCache initializes listenerCache if nil
func (a *StdManagerAdapter[T]) ensureCache() {
	if a.listenerCache == nil {
		a.listenerCache = a.Mgr.GetListenerCache()
	}
}

func (a *StdManagerAdapter[T]) AddEvent(e Event[T]) {
	a.Mgr.AddEvent(&EventTToAnyAdapter[T]{OriginalEvent: e})
}

func (a *StdManagerAdapter[T]) On(name string, listener Listener[T], priority ...int) {
	if listener == nil {
		panicf("event: the event %q listener cannot be empty", name)
	}

	a.ensureCache()

	wrapped := wrapListenerForType[T](a, listener, false)

	// Store the mapping between original and wrapped listener
	a.listenerCache.StoreWrapped(listener.ID(), wrapped)

	a.Mgr.On(name, wrapped, priority...)
}

// wrapListenerForType creates a wrapped Listener[any] that converts to Listener[T]
// If once is true, the listener will be removed after first execution
// mgr can be either EventManager[T] or Subscriber[T]
func wrapListenerForType[T any](mgr any, listener Listener[T], once bool) Listener[any] {
	return NewListenerFunc[any](func(e Event[any]) error {
		dataAsT, ok := e.Data().(T)
		if !ok {
			var zeroT T
			return fmt.Errorf("event: data type mismatch for event '%s'. Listener expected type %T, but event data is type %T",
				e.Name(), zeroT, e.Data())
		}

		if once {
			switch v := mgr.(type) {
			case *StdManagerAdapter[T]:
				v.RemoveListener(e.Name(), listener)
			case EventManager[T]:
				v.RemoveListener(e.Name(), listener)
			}
		}

		return listener.Handle(newBasicEventAdapter(e, dataAsT))
	})
}

// wrapListener creates a wrapped Listener[any] that safely handles type conversion
func (a *StdManagerAdapter[T]) wrapListener(listener Listener[T]) Listener[any] {
	if listener == nil {
		return nil
	}

	a.ensureCache()

	// Check if we already have a wrapper
	if wrapped, ok := a.listenerCache.GetWrapped(listener.ID()); ok {
		return wrapped
	}

	wrapped := wrapListenerForType[T](a, listener, false)

	// Store the mapping
	a.listenerCache.StoreWrapped(listener.ID(), wrapped)

	return wrapped
}

func (a *StdManagerAdapter[T]) Listen(name string, listener Listener[T], priority ...int) {
	Listen(name, listener, priority...)
}

func (a *StdManagerAdapter[T]) Once(name string, listener Listener[T], priority ...int) {
	Once(name, listener, priority...)
}

func (a *StdManagerAdapter[T]) Fire(name string, data T) (error, Event[T]) {
	return Fire(name, data)
}

func (a *StdManagerAdapter[T]) Trigger(name string, data T) (error, Event[T]) {
	return Trigger(name, data)
}

func (a *StdManagerAdapter[T]) MustFire(name string, data T) Event[T] {
	return MustFire(name, data)
}

func (a *StdManagerAdapter[T]) MustTrigger(name string, data T) Event[T] {
	return MustTrigger(name, data)
}

func (a *StdManagerAdapter[T]) FireBatch(events ...any) []error {
	// Convert each event from Event[T] to Event[any] before passing to std manager
	converted := make([]any, len(events))
	for i, e := range events {
		if evt, ok := e.(Event[T]); ok {
			converted[i] = NewEventTToAnyAdapter(evt)
		} else {
			converted[i] = e
		}
	}
	return a.Mgr.FireBatch(converted...)
}

func (a *StdManagerAdapter[T]) FireEvent(e Event[T]) error {
	return FireEvent(e)
}

func (a *StdManagerAdapter[T]) TriggerEvent(e Event[T]) error {
	return TriggerEvent(e)
}

func (a *StdManagerAdapter[T]) Async(name string, data T) {
	a.FireC(name, data)
}

func (a *StdManagerAdapter[T]) FireC(name string, data T) {
	Async(name, data)
}

func (a *StdManagerAdapter[T]) FireAsync(e Event[T]) {
	FireAsync(e)
}

func (a *StdManagerAdapter[T]) AsyncFire(e Event[T]) {
	AsyncFire(e)
}

func (a *StdManagerAdapter[T]) FireTyped(name string, data T) (error, Event[T]) {
	return FireTyped[T](a.Mgr, name, data)
}

func (a *StdManagerAdapter[T]) AwaitFire(e Event[T]) error {
	return a.Mgr.AwaitFire(NewEventTToAnyAdapter(e))
}

func (a *StdManagerAdapter[T]) CloseWait() error {
	return CloseWait()
}

func (a *StdManagerAdapter[T]) Close() error {
	return a.Mgr.Close()
}

func (a *StdManagerAdapter[T]) AddListener(name string, listener Listener[T], priority ...int) {
	a.On(name, listener, priority...)
}

func (a *StdManagerAdapter[T]) RemoveListener(name string, listener Listener[T]) {
	a.ensureCache()

	// Try to get the wrapped version from cache first
	if wrapped, ok := a.listenerCache.GetWrapped(listener.ID()); ok {
		a.Mgr.RemoveListener(name, wrapped)
		a.listenerCache.DeleteWrapped(listener.ID())
		return
	}

	// If not in cache, wrap and remove (this handles cases where cache was cleared)
	wrapped := wrapListenerForType[T](a, listener, false)
	a.Mgr.RemoveListener(name, wrapped)
	a.listenerCache.DeleteWrapped(listener.ID())
}

func (a *StdManagerAdapter[T]) RemoveListeners(name string) {
	a.Mgr.RemoveListeners(name)
}

func (a *StdManagerAdapter[T]) HasListeners(name string) bool {
	return HasListeners(name)
}

func (a *StdManagerAdapter[T]) ListenersCount(name string) int {
	return a.Mgr.ListenersCount(name)
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
			wrappedEvents[name] = wrapListenerForType[T](sw.Subscriber, lt, false)
		case ListenerItem[T]:
			wrappedEvents[name] = ListenerItem[any]{
				Priority: lt.Priority,
				Listener: wrapListenerForType[T](sw.Subscriber, lt.Listener, false),
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

// getManager extracts the underlying *Manager[any] from an EventManager[any]
func getManager(em EventManager[any]) *Manager[any] {
	switch v := em.(type) {
	case *Manager[any]:
		return v
	case ManagerGetter:
		return v.GetMgr()
	default:
		panic("event: requires *Manager[any] or an implementation of ManagerGetter")
	}
}

// OnTyped registers a typed listener with the given Manager[any]
func OnTyped[P any](em EventManager[any], name string, listener Listener[P], priority ...int) {
	if listener == nil {
		panicf("event: the event %q listener cannot be empty", name)
	}

	mgr := getManager(em)
	adapter := ForType[P, any](mgr).(*StdManagerAdapter[P])
	adapter.On(name, listener, priority...)
}

// RemoveTypedListener removes a typed listener from the given Manager[any]
func RemoveTypedListener[P any](em EventManager[any], name string, listener Listener[P]) {
	mgr := getManager(em)
	adapter := ForType[P, any](mgr).(*StdManagerAdapter[P])
	adapter.RemoveListener(name, listener)
}

// FireTyped fires an event with typed data on the given Manager[any]
func FireTyped[P any](em EventManager[any], name string, data P) (error, Event[P]) {
	err, evtAny := em.Fire(name, data)
	if err != nil {
		return err, nil
	}

	dataAsP, ok := evtAny.Data().(P)
	if !ok {
		var zeroP P
		return fmt.Errorf("event: data type mismatch after firing event '%s'. Expected %T, got %T",
			evtAny.Name(), zeroP, evtAny.Data()), nil
	}

	return err, newBasicEventAdapter(evtAny, dataAsP)
}

// NewMEvent creates a new BasicEvent with event.M data type
func NewMEvent(name string, data M) *BasicEvent[M] {
	return NewBasic(name, data)
}

// ForType returns a EventManager[T] adapter for the given manager
func ForType[T any, M any](mgr *Manager[any]) EventManager[T] {
	// Get the reflect.Type of T safely, even for interfaces
	typ := reflect.TypeOf((*T)(nil)).Elem()

	// Get manager-specific cache
	cache := mgr.GetAdapterCache()

	// Load or create the adapter for this type
	if v, ok := cache.Load(typ); ok {
		return v.(*StdManagerAdapter[T])
	}

	adapter := NewStdManagerAdapter[T](mgr)
	cache.Store(typ, adapter)
	return adapter
}

// StdForType returns a EventManager[T] adapter for the default std manager
func StdForType[T any]() EventManager[T] {
	return ForType[T, any](std)
}

func (a *StdManagerAdapter[T]) AddEventFc(name string, fc FactoryFunc[T]) {
	a.Mgr.AddEventFc(name, func() Event[any] {
		e := fc()
		return NewEventTToAnyAdapter(e)
	})
}

func (a *StdManagerAdapter[T]) GetEvent(name string) (Event[T], bool) {
	e, ok := a.Mgr.GetEvent(name)
	if !ok {
		return nil, false
	}

	// Convert Event[any] to Event[T]
	if adapter, ok := e.(*EventTToAnyAdapter[T]); ok {
		return adapter.OriginalEvent, true
	}

	// Handle case where event is not already wrapped
	dataAsT, ok := e.Data().(T)
	if !ok {
		return nil, false
	}
	return newBasicEventAdapter(e, dataAsT), true
}

func (a *StdManagerAdapter[T]) HasEvent(name string) bool {
	return a.Mgr.HasEvent(name)
}

func (a *StdManagerAdapter[T]) RemoveEvent(name string) {
	a.Mgr.RemoveEvent(name)
}

func (a *StdManagerAdapter[T]) RemoveEvents() {
	a.Mgr.RemoveEvents()
}

func (a *StdManagerAdapter[T]) Reset() {
	a.Mgr.Reset()
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

	// Get StdManagerAdapter instance for type T
	adapter := StdForType[T]().(*StdManagerAdapter[T])
	wrappedListener := adapter.wrapListener(listener)
	std.On(name, wrappedListener, priority...)
}

// OnFunc registers a function listener to the event
func OnFunc[T any](name string, fn EventHandlerFunc[T], priority ...int) {
	if fn == nil {
		panicf("event: the event %q listener function cannot be nil", name)
	}

	On(name, NewListenerFunc[T](fn), priority...)
}

// Once register a listener to the event. trigger once
func Once[T any](name string, listener Listener[T], priority ...int) {
	if listener == nil {
		panicf("event: the event %q listener cannot be empty", name)
	}

	// Get StdManagerAdapter instance for type T
	adapter := StdForType[T]().(*StdManagerAdapter[T])
	wrappedListener := wrapListenerForType[T](adapter, listener, true)

	// Store the mapping between original and wrapped listener
	adapter.listenerCache.StoreWrapped(listener.ID(), wrappedListener)

	std.On(name, wrappedListener, priority...)
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

		wrappedListeners[listenerName] = NewListenerFunc[any](func(e Event[any]) error {
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
	adaptedEventForStd := NewEventTToAnyAdapter(e)
	std.AsyncFire(adaptedEventForStd)
}

// Async fire event by channel
func Async[T any](name string, data T) {
	std.Async(name, data) // Manager[any].Async takes (string, any), data T is compatible
}

// FireAsync fire event by channel
func FireAsync[T any](e Event[T]) {
	adaptedEventForStd := NewEventTToAnyAdapter(e)
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
	adaptedEventForStd := NewEventTToAnyAdapter(e)
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
	adaptedEventForStd := NewEventTToAnyAdapter(e)
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
