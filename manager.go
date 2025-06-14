package event

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

const (
	defaultChannelSize = 100
	defaultConsumerNum = 3
)

// Manager implements EventManager interface for managing events and listeners
type Manager[T any] struct {
	Options
	sync.Mutex

	wg  sync.WaitGroup
	ch  chan Event[T]
	oc  sync.Once
	err error // latest error

	// name of the manager
	name string
	// pool sync.Pool
	// is a sample for new BasicEvent
	sample *BasicEvent[T]

	// storage user pre-defined event factory func.
	eventFc map[string]FactoryFunc[T]

	// storage all event name and ListenerQueue map
	listeners map[string]*ListenerQueue[T]
	// storage all event names by listened
	listenedNames map[string]int
}

// NewM create event manager with type any. Alias of NewManager[any]().
func NewM(name string, fns ...OptionFn) *Manager[any] {
	return NewManager[any](name, fns...)
}

// NewManager create event manager
func NewManager[T any](name string, fns ...OptionFn) *Manager[T] {
	em := &Manager[T]{
		name:   name,
		sample: &BasicEvent[T]{},
		// events storage
		eventFc: make(map[string]FactoryFunc[T]),
		// listeners
		listeners:     make(map[string]*ListenerQueue[T]),
		listenedNames: make(map[string]int),
	}

	// em.EnableLock = true
	// for async fire by goroutine
	em.ConsumerNum = defaultConsumerNum
	em.ChannelSize = defaultChannelSize

	// apply options
	return em.WithOptions(fns...)
}

// WithOptions create event manager with options
func (em *Manager[T]) WithOptions(fns ...OptionFn) *Manager[T] {
	for _, fn := range fns {
		fn(&em.Options)
	}
	return em
}

/*************************************************************
 * -- register listeners
 *************************************************************/

// AddListener register a event handler/listener. alias of the method On()
func (em *Manager[T]) AddListener(name string, listener Listener[T], priority ...int) {
	em.On(name, listener, priority...)
}

// Listen register a event handler/listener. alias of the On()
func (em *Manager[T]) Listen(name string, listener Listener[T], priority ...int) {
	em.On(name, listener, priority...)
}

// Listen register a event handler/listener. trigger once.
func (em *Manager[T]) Once(name string, listener Listener[T], priority ...int) {
	var listenerOnce Listener[T]
	listenerOnce = ListenerFunc[T](func(e Event[T]) error {
		em.RemoveListener(name, listenerOnce)
		return listener.Handle(e)
	})
	em.On(name, listenerOnce, priority...)
}

// On register a event handler/listener. can setting priority.
//
// Usage:
//
//	em.On("evt0", listener)
//	em.On("evt0", listener, High)
func (em *Manager[T]) On(name string, listener Listener[T], priority ...int) {
	pv := Normal
	if len(priority) > 0 {
		pv = priority[0]
	}

	em.addListenerItem(name, &ListenerItem[T]{pv, listener})
}

// AddListeners registers multiple listeners at once.
func (em *Manager[T]) AddListeners(listeners map[string]Listener[T], priority ...int) {
	pv := Normal
	if len(priority) > 0 {
		pv = priority[0]
	}
	for name, listener := range listeners {
		em.addListenerItem(name, &ListenerItem[T]{pv, listener})
	}
}

func (em *Manager[T]) addListenerItem(name string, li *ListenerItem[T]) {
	name = goodName(name, true)
	if li.Listener == nil {
		panicf("event: the event %q listener cannot be empty", name)
	}
	if reflect.ValueOf(li.Listener).Kind() == reflect.Struct {
		panicf("event: %q - struct listener must be pointer", name)
	}

	// exists, append it.
	if lq, ok := em.listeners[name]; ok {
		lq.Push(li)
	} else { // first add.
		em.listenedNames[name] = 1
		em.listeners[name] = (&ListenerQueue[T]{}).Push(li)
	}
}

/*************************************************************
 * Listener Manage: - trigger event
 *************************************************************/

// MustTrigger alias of the method MustFire()
func (em *Manager[T]) MustTrigger(name string, data T) Event[T] {
	return em.MustFire(name, data)
}

// MustFire fire event by name. will panic on error
func (em *Manager[T]) MustFire(name string, data T) Event[T] {
	err, e := em.Fire(name, data)
	if err != nil {
		panic(err)
	}
	return e
}

// Trigger alias of the method Fire()
func (em *Manager[T]) Trigger(name string, data T) (error, Event[T]) {
	return em.Fire(name, data)
}

// TriggerEvent is an alias for FireEvent()
func (em *Manager[T]) TriggerEvent(e Event[T]) error {
	return em.FireEvent(e)
}

// Fire trigger event by name. if not found listener, will return (nil, nil)
func (em *Manager[T]) Fire(name string, data T) (err error, e Event[T]) {
	// call listeners handle event
	e, err = em.fireByName(name, data, false)
	return
}

// Async fire event by go channel.
//
// Note: if you want to use this method, you should
// call the method Close() after all events are fired.
func (em *Manager[T]) Async(name string, data T) {
	_, _ = em.fireByName(name, data, true)
}

// FireC async fire event by go channel. alias of the method Async()
//
// Note: if you want to use this method, you should
// call the method Close() after all events are fired.
func (em *Manager[T]) FireC(name string, data T) {
	_, _ = em.fireByName(name, data, true)
}

// fire event by name.
//
// if useCh is true, will async fire by channel. always return (nil, nil)
//
// On useCh=false:
//   - will call listeners handle event.
//   - if not found listener, will return (nil, nil)
func (em *Manager[T]) fireByName(name string, params T, useCh bool) (e Event[T], err error) {
	name = goodName(name, false)

	// use pre-defined Event
	if fc, ok := em.eventFc[name]; ok {
		e = fc() // make new instance
		// Check if params is non-zero using reflection
		if !reflect.ValueOf(params).IsZero() {
			e.SetData(params)
		}
	} else {
		// create new basic event instance
		e = em.newBasicEvent(name, params)
	}

	// fire by channel
	if useCh {
		em.FireAsync(e)
		return nil, nil
	}

	// call listeners handle event
	err = em.FireEvent(e)
	return
}

// FireEvent fire event by given Event instance
func (em *Manager[T]) FireEvent(e Event[T]) (err error) {
	if em.EnableLock {
		em.Lock()
		defer em.Unlock()
	}

	// ensure aborted is false.
	e.Abort(false)
	name := e.Name()

	// fire group listeners by wildcard. eg "db.user.*"
	if em.MatchMode == ModePath {
		err = em.firePathMode(name, e)
		return
	}

	// handle mode: ModeSimple
	err = em.fireSimpleMode(name, e)
	if err != nil || e.IsAborted() {
		return
	}

	// fire wildcard event listeners
	if lq, ok := em.listeners[Wildcard]; ok {
		for _, li := range lq.Sort().Items() {
			err = li.Listener.Handle(e)
			if err != nil || e.IsAborted() {
				break
			}
		}
	}
	return
}

// ModeSimple has group listeners by wildcard. eg "db.user.*"
//
// Example:
//   - event "db.user.add" will trigger listeners on the "db.user.*"
func (em *Manager[T]) fireSimpleMode(name string, e Event[T]) (err error) {
	// fire direct matched listeners. eg: db.user.add
	if lq, ok := em.listeners[name]; ok {
		// sort by priority before call.
		for _, li := range lq.Sort().Items() {
			err = li.Listener.Handle(e)
			if err != nil || e.IsAborted() {
				return
			}
		}
	}

	pos := strings.LastIndexByte(name, '.')

	if pos > 0 && pos < len(name) {
		groupName := name[:pos+1] + Wildcard // "app.*"

		if lq, ok := em.listeners[groupName]; ok {
			for _, li := range lq.Sort().Items() {
				err = li.Listener.Handle(e)
				if err != nil || e.IsAborted() {
					return
				}
			}
		}
	}

	return nil
}

// ModePath fire group listeners by ModePath.
//
// Example:
//   - event "db.user.add" will trigger listeners on the "db.**"
//   - event "db.user.add" will trigger listeners on the "db.user.*"
func (em *Manager[T]) firePathMode(name string, e Event[T]) (err error) {
	for pattern, lq := range em.listeners {
		if pattern == name || matchNodePath(pattern, name, ".") {
			for _, li := range lq.Sort().Items() {
				err = li.Listener.Handle(e)
				if err != nil || e.IsAborted() {
					return
				}
			}
		}
	}

	return nil
}

/*************************************************************
 * Fire by channel
 *************************************************************/

// FireAsync async fire event by go channel.
//
// Note: if you want to use this method, you should
// call the method Close() after all events are fired.
//
// Example:
//
//	em := NewManager("test")
//	em.FireAsync("db.user.add", M{"id": 1001})
func (em *Manager[T]) FireAsync(e Event[T]) {
	// once make consumers
	em.oc.Do(func() {
		em.makeConsumers()
	})

	// dispatch event
	em.ch <- e
}

// async fire event by 'go' keywords
func (em *Manager[T]) makeConsumers() {
	if em.ConsumerNum <= 0 {
		em.ConsumerNum = defaultConsumerNum
	}
	if em.ChannelSize <= 0 {
		em.ChannelSize = defaultChannelSize
	}

	em.ch = make(chan Event[T], em.ChannelSize)

	// make event consumers
	for i := 0; i < em.ConsumerNum; i++ {
		em.wg.Add(1)

		go func() {
			defer func() {
				if err := recover(); err != nil {
					em.err = fmt.Errorf("async consum event error: %v", err)
				}
				em.wg.Done()
			}()

			// keep running until channel closed
			for e := range em.ch {
				_ = em.FireEvent(e) // ignore async fire error
			}
		}()
	}
}

// CloseWait close channel and wait all async event done.
func (em *Manager[T]) CloseWait() error {
	if err := em.Close(); err != nil {
		return err
	}
	return em.Wait()
}

// Wait wait all async event done.
func (em *Manager[T]) Wait() error {
	em.wg.Wait()
	return em.err
}

// Close event channel, deny to fire new event.
func (em *Manager[T]) Close() error {
	if em.ch != nil {
		close(em.ch)
	}
	return nil
}

// FireBatch fire multi event at once.
func (em *Manager[T]) FireBatch(es ...any) (ers []error) {
	var err error
	var tEvent Event[T]
	var evt Event[T]
	var evtM Event[M]
	var ok bool
	var name string
	for _, e := range es {
		if name, ok = e.(string); ok {
			err, _ = em.Fire(name, *new(T))
		} else if evt, ok = e.(Event[T]); ok {
			err = em.FireEvent(evt)
		} else if evtM, ok = e.(Event[M]); ok {
			// Convert M event to T event using helper
			tEvent, err = ConvertEvent[M, T](evtM)
			if err != nil {
				ers = append(ers, err)
				continue
			}
			err = em.FireEvent(tEvent)
		}
		if err != nil {
			ers = append(ers, err)
		}
	}
	return
}

// AsyncFire simple async fire event by 'go' keywords
func (em *Manager[T]) AsyncFire(e Event[T]) {
	go func(e Event[T]) {
		_ = em.FireEvent(e)
	}(e)
}

// AwaitFire async fire event by 'go' keywords, but will wait return result
func (em *Manager[T]) AwaitFire(e Event[T]) (err error) {
	ch := make(chan error)

	go func(e Event[T]) {
		err := em.FireEvent(e)
		ch <- err
	}(e)

	err = <-ch
	close(ch)
	return
}

/*************************************************************
 * Event Manage
 *************************************************************/

// AddEvent add a pre-defined event instance to manager.
func (em *Manager[T]) AddEvent(e Event[T]) {
	name := goodName(e.Name(), false)

	var ok bool
	_, ok = e.(Cloneable[T])

	if ok {
		ec := e.(Cloneable[T])
		em.AddEventFc(name, func() Event[T] {
			return ec.Clone()
		})
	} else {
		em.AddEventFc(name, func() Event[T] {
			return e
		})
	}
}

// AddEventFc add a pre-defined event factory func to manager.
func (em *Manager[T]) AddEventFc(name string, fc FactoryFunc[T]) {
	em.Lock()
	em.eventFc[name] = fc
	em.Unlock()
}

// GetEvent get a pre-defined event instance by name
func (em *Manager[T]) GetEvent(name string) (e Event[T], ok bool) {
	fc, ok := em.eventFc[name]
	if ok {
		e = fc()
		return
	}
	return
}

// HasEvent has pre-defined event check
func (em *Manager[T]) HasEvent(name string) bool {
	_, ok := em.eventFc[name]
	return ok
}

// RemoveEvent delete pre-define Event by name
func (em *Manager[T]) RemoveEvent(name string) {
	if _, ok := em.eventFc[name]; ok {
		delete(em.eventFc, name)
	}
}

// RemoveEvents remove all registered events
func (em *Manager[T]) RemoveEvents() {
	em.eventFc = make(map[string]FactoryFunc[T])
}

/*************************************************************
 * Helper Methods
 *************************************************************/

// newBasicEvent create new BasicEvent by clone em.sample
func (em *Manager[T]) newBasicEvent(name string, data T) *BasicEvent[T] {
	var cp = *em.sample

	cp.SetName(name)
	cp.SetData(data)
	return &cp
}

// HasListeners check has direct listeners for the event name.
func (em *Manager[T]) HasListeners(name string) bool {
	_, ok := em.listenedNames[name]
	return ok
}

// Listeners get all listeners
func (em *Manager[T]) Listeners() map[string]*ListenerQueue[T] {
	return em.listeners
}

// ListenersByName get listeners by given event name
func (em *Manager[T]) ListenersByName(name string) *ListenerQueue[T] {
	return em.listeners[name]
}

// ListenersCount get listeners number for the event name.
func (em *Manager[T]) ListenersCount(name string) int {
	if lq, ok := em.listeners[name]; ok {
		return lq.Len()
	}
	return 0
}

// ListenedNames get listened event names
func (em *Manager[T]) ListenedNames() map[string]int {
	return em.listenedNames
}

// RemoveListener remove a given listener, you can limit event name.
//
// Usage:
//
//	RemoveListener("", listener)
//	RemoveListener("name", listener) // limit event name.
func (em *Manager[T]) RemoveListener(name string, listener Listener[T]) {
	if name != "" {
		if lq, ok := em.listeners[name]; ok {
			lq.Remove(listener)

			// delete from manager
			if lq.IsEmpty() {
				delete(em.listeners, name)
				delete(em.listenedNames, name)
			}
		}
		return
	}

	// name is empty. find all listener and remove matched.
	for name, lq := range em.listeners {
		lq.Remove(listener)

		// delete from manager
		if lq.IsEmpty() {
			delete(em.listeners, name)
			delete(em.listenedNames, name)
		}
	}
}

// RemoveListeners remove listeners by given name
func (em *Manager[T]) RemoveListeners(name string) {
	_, ok := em.listenedNames[name]
	if ok {
		em.listeners[name].Clear()

		// delete from manager
		delete(em.listeners, name)
		delete(em.listenedNames, name)
	}
}

// Clear alias of the Reset()
// Clear is an alias for Reset()
func (em *Manager[T]) Clear() { em.Reset() }

// Subscribe adds all listeners from a subscriber
func (em *Manager[T]) Subscribe(sbr Subscriber[T]) {
	events := sbr.SubscribedEvents()
	for name, listener := range events {
		l, priority, err := convertListener[T](listener)
		if err != nil {
			panicf("event: invalid listener type %T for event '%s': %v", listener, name, err)
		}
		em.On(name, l, priority)
	}
}

// convertListener converts a listener to the target type T with optional priority
func convertListener[T any](listener any) (Listener[T], int, error) {
	switch lt := listener.(type) {
	case Listener[T]:
		return lt, Normal, nil
	case *Listener[T]:
		return *lt, Normal, nil
	case ListenerItem[T]:
		return lt.Listener, lt.Priority, nil
	case Listener[M]:
		return ConvertListener[M, T](lt), Normal, nil
	case ListenerItem[M]:
		return ConvertListener[M, T](lt.Listener), lt.Priority, nil
	default:
		rt := reflect.TypeOf(listener)
		if rt != nil && rt.Kind() == reflect.Ptr {
			// Try pointer types
			if l, ok := listener.(Listener[M]); ok {
				return ConvertListener[M, T](l), Normal, nil
			}
			if l, ok := listener.(Listener[T]); ok {
				return l, Normal, nil
			}
		}
		return nil, 0, fmt.Errorf("unsupported listener type")
	}
}

// Reset the manager, clear all data.
// Reset clears all listeners and events
func (em *Manager[T]) Reset() {
	// clear all listeners
	for _, lq := range em.listeners {
		lq.Clear()
	}

	// reset all
	em.ch = nil
	em.oc = sync.Once{}
	em.wg = sync.WaitGroup{}

	em.eventFc = make(map[string]FactoryFunc[T])
	em.listeners = make(map[string]*ListenerQueue[T])
	em.listenedNames = make(map[string]int)
}
