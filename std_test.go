package event_test

import (
	"bytes"
	"fmt"
	"github.com/gookit/goutil/testutil/assert"
	"go.lumeweb.com/event/v2"
	"sync"
	"testing"
)

var emptyListener = func(e event.Event[event.M]) error {
	return nil
}

func TestAddEvent(t *testing.T) {
	defer event.Reset()
	event.Std().RemoveEvents()

	// no name
	assert.Panics(t, func() {
		evt := event.NewBasic[event.M]("", event.M{})
		evt.SetName("") // explicitly set empty name
		event.AddEvent[event.M](evt)
	})

	_, ok := event.GetEvent("evt1")
	assert.False(t, ok)

	// event.AddEvent
	e := event.NewBasic("evt1", event.M{"k1": "inhere"})
	event.AddEvent[event.M](e)
	// add by AttachTo
	event.NewMEvent("evt2", event.M{}).AttachTo(event.StdForType[event.M]())

	assert.False(t, e.IsAborted())
	assert.True(t, event.HasEvent("evt1"))
	assert.True(t, event.HasEvent("evt2"))
	assert.False(t, event.HasEvent("not-exist"))

	// event.GetEvent
	r1, ok := event.GetEvent("evt1")
	assert.True(t, ok)

	// Handle case where event is wrapped in adapter
	if adapter, ok := r1.(*event.EventTToAnyAdapter[event.M]); ok {
		assert.Equal(t, e, adapter.OriginalEvent)
	} else {
		assert.Equal(t, e, r1)
	}

	// RemoveEvent
	event.Std().RemoveEvent("evt2")
	assert.False(t, event.HasEvent("evt2"))

	// RemoveEvents
	event.Std().RemoveEvents()
	assert.False(t, event.HasEvent("evt1"))
}

func TestAddEventFc(t *testing.T) {
	// clear all
	event.Reset()
	event.AddEvent[event.M](&testEvent{
		name: "evt1",
	})
	assert.True(t, event.HasEvent("evt1"))

	event.AddEventFc("test", func() event.Event[any] {
		return event.NewBasic[any]("test", nil)
	})

	assert.True(t, event.HasEvent("test"))
}

func TestFire(t *testing.T) {
	buf := new(bytes.Buffer)
	fn := func(e event.Event[event.M]) error {
		_, _ = fmt.Fprintf(buf, "event: %s", e.Name())
		return nil
	}

	event.On[event.M]("evt1", event.NewListenerFunc[event.M](fn), 0)
	event.On[event.M]("evt1", event.NewListenerFunc[event.M](emptyListener), event.High)
	assert.True(t, event.HasListeners("evt1"))

	err, e := event.Fire[event.M]("evt1", event.M{})
	assert.NoError(t, err)
	assert.Equal(t, "evt1", e.Name())
	assert.Equal(t, "event: evt1", buf.String())

	event.NewBasic("evt2", event.M{}).AttachTo(&event.StdManagerAdapter[event.M]{Mgr: event.Std()})
	event.On[event.M]("evt2", event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		assert.Equal(t, "evt2", e.Name())
		assert.Equal(t, "v", e.Data()["k"])
		return nil
	}), event.AboveNormal)

	assert.True(t, event.HasListeners("evt2"))
	err, e = event.Trigger[event.M]("evt2", event.M{"k": "v"})
	assert.NoError(t, err)
	assert.Equal(t, "evt2", e.Name())
	assert.Equal(t, map[string]any{"k": "v"}, e.Data())

	// clear all
	event.Reset()
	assert.False(t, event.HasListeners("evt1"))
	assert.False(t, event.HasListeners("evt2"))

	err, e = event.Fire("not-exist", event.M{})
	assert.NoError(t, err)
	assert.NotEmpty(t, e)
}

func TestAddSubscriber(t *testing.T) {
	event.Std().Subscribe(&testSubscriber{})

	assert.True(t, event.HasListeners("e1"))
	assert.True(t, event.HasListeners("e2"))
	assert.True(t, event.HasListeners("e3"))

	ers := event.FireBatch("e1", event.NewBasic("e2", event.M{}))
	assert.Len(t, ers, 1)

	assert.Panics(t, func() {
		event.Subscribe[any](event.Std(), testSubscriber2{})
	})

	event.Reset()
}

func TestFireEvent(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	var wg sync.WaitGroup

	evt1 := event.NewBasic("evt1", event.M{"n": "inhere"})
	event.AddEvent[event.M](evt1)

	assert.True(t, event.HasEvent("evt1"))
	assert.False(t, event.HasEvent("not-exist"))

	// Clear any existing listeners first
	event.Reset()

	listener := event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		wg.Done()
		_, _ = fmt.Fprintf(buf, "event: %s, params: n=%s", e.Name(), e.Data()["n"])
		return nil
	})

	event.Listen[event.M]("evt1", listener, event.Normal)

	assert.True(t, event.HasListeners("evt1"))
	assert.False(t, event.HasListeners("not-exist"))

	wg.Add(1)
	err := event.FireEvent[event.M](evt1)
	assert.NoError(t, err)
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
	buf.Reset()

	wg.Add(1)
	err = event.TriggerEvent[event.M](evt1)
	assert.NoError(t, err)
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
	buf.Reset()

	// Use same listener to avoid duplicate registration
	event.On[event.M]("evt1", listener)
	wg.Add(1)

	event.AsyncFire[event.M](evt1)
	wg.Wait() // Wait for listener to complete
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
}

func TestAsync(t *testing.T) {
	event.Reset()
	event.Config(event.UsePathMode)

	buf := new(bytes.Buffer)
	event.On[event.M]("test", event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		buf.WriteString("test:")
		buf.WriteString(fmt.Sprintf("%v", e.Data()["key"]))
		buf.WriteString("|")
		return nil
	}))

	var wg sync.WaitGroup
	wg.Add(2)

	event.On[event.M]("test", event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		defer wg.Done()
		buf.WriteString("test:")
		buf.WriteString(fmt.Sprintf("%v", e.Data()["key"]))
		buf.WriteString("|")
		return nil
	}))

	event.Async("test", event.M{"key": "val1"})
	te := &testEvent{name: "test", data: event.M{"key": "val2"}}
	event.FireAsync[event.M](te)
	wg.Wait() // Wait for both events to be processed
	s := buf.String()
	assert.Contains(t, s, "test:val1|")
	assert.Contains(t, s, "test:val2|")
}

func TestFire_point_at_end(t *testing.T) {
	// clear all
	event.Reset()
	event.Listen[event.M]("db.*", event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		e.SetData(event.M{"key": "val"})
		return nil
	}))

	err, e := event.Fire[event.M]("db.add", event.M{})
	assert.NoError(t, err)
	assert.Equal(t, "val", e.Data()["key"])

	err, e = event.Fire("db", event.M{})
	assert.NotEmpty(t, e)
	assert.NoError(t, err)

	err, e = event.Fire("db.", event.M{})
	assert.NotEmpty(t, e)
	assert.NoError(t, err)
}

func TestFire_notExist(t *testing.T) {
	// clear all
	event.Reset()

	err, e := event.Fire("not-exist", event.M{})
	assert.NoError(t, err)
	assert.NotEmpty(t, e)
}

func TestMustFire(t *testing.T) {
	defer event.Reset()

	event.On[event.M]("n1", event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
		return fmt.Errorf("an error")
	}), event.Max)
	event.On[event.M]("n2", event.NewListenerFunc[event.M](emptyListener), event.Min)

	assert.Panics(t, func() {
		_ = event.MustFire[event.M]("n1", event.M{})
	})

	assert.NotPanics(t, func() {
		_ = event.MustTrigger[event.M]("n2", event.M{})
	})
}

func TestOn(t *testing.T) {
	defer event.Reset()

	assert.Panics(t, func() {
		event.On[event.M]("", event.NewListenerFunc[event.M](emptyListener), 0)
	})
	assert.Panics(t, func() {
		event.On[event.M]("name", nil, 0)
	})
	assert.Panics(t, func() {
		event.On[event.M]("++df", event.NewListenerFunc[event.M](emptyListener), 0)
	})

	std := event.Std()
	event.On[event.M]("n1", event.NewListenerFunc[event.M](emptyListener), event.Min)
	assert.Equal(t, 1, std.ListenersCount("n1"))
	assert.Equal(t, 0, std.ListenersCount("not-exist"))
	assert.True(t, event.HasListeners("n1"))
	assert.False(t, event.HasListeners("name"))

	assert.NotEmpty(t, std.Listeners())
	assert.NotEmpty(t, std.ListenersByName("n1"))

	std.RemoveListeners("n1")
	assert.False(t, event.HasListeners("n1"))
}

func TestOnTyped(t *testing.T) {
	defer event.Reset()

	em := event.Std()
	buf := new(bytes.Buffer)

	// Register typed listener expecting string data
	listener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("got:" + e.Data())
		return nil
	})

	// Register using OnTyped with string type
	event.OnTyped[string](em, "str_evt", listener, event.Normal)
	assert.Equal(t, 1, em.ListenersCount("str_evt"))

	// FireTyped with matching type
	err, evt := event.FireTyped[string](em, "str_evt", "test data")
	assert.NoError(t, err)
	assert.Equal(t, "got:test data", buf.String())
	assert.Equal(t, "str_evt", evt.Name())
	buf.Reset()

	// Test type mismatch by registering a listener expecting int but firing with string
	intListener := event.NewListenerFunc[int](func(e event.Event[int]) error {
		buf.WriteString("int-handler:" + fmt.Sprint(e.Data()))
		return nil
	})
	event.OnTyped[int](em, "int_evt", intListener)

	err, evt = event.FireTyped[string](em, "int_evt", "should-be-int")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "data type mismatch")
	assert.Nil(t, evt)
	assert.Empty(t, buf.String())
}

func TestRemoveTypedListener(t *testing.T) {
	defer event.Reset()

	em := event.Std()
	buf := new(bytes.Buffer)

	// Register typed listener expecting string data
	listener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("got:" + e.Data())
		return nil
	})

	// Register using OnTyped
	event.OnTyped[string](em, "str_evt", listener, event.Normal)
	assert.Equal(t, 1, em.ListenersCount("str_evt"))

	// Remove the listener
	event.RemoveTypedListener[string](em, "str_evt", listener)
	assert.Equal(t, 0, em.ListenersCount("str_evt"))

	// FireTyped - should not trigger listener but still return event
	err, evt := event.FireTyped[string](em, "str_evt", "test data")
	assert.NoError(t, err)
	assert.NotNil(t, evt)
	assert.Equal(t, "str_evt", evt.Name())
	assert.Empty(t, buf.String())
}

func TestRemoveTypedListener_WithMultipleListeners(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	// Register two typed listeners
	listener1 := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("listener1:" + e.Data())
		return nil
	})
	listener2 := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("listener2:" + e.Data())
		return nil
	})

	em := event.Std()
	event.OnTyped[string](em, "multi_evt", listener1)
	event.OnTyped[string](em, "multi_evt", listener2)
	assert.Equal(t, 2, em.ListenersCount("multi_evt"))

	// Remove only listener1
	event.RemoveTypedListener[string](em, "multi_evt", listener1)
	assert.Equal(t, 1, em.ListenersCount("multi_evt"))

	// Fire event - only listener2 should trigger
	err, _ := event.FireTyped[string](em, "multi_evt", "test")
	assert.NoError(t, err)
	assert.Equal(t, "listener2:test", buf.String())
}

func TestRemoveTypedListener_WithDifferentTypes(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	// Register listeners with different types
	stringListener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("string:" + e.Data())
		return nil
	})
	intListener := event.NewListenerFunc[int](func(e event.Event[int]) error {
		buf.WriteString(fmt.Sprintf("int:%d", e.Data()))
		return nil
	})

	em := event.Std()
	event.OnTyped[string](em, "type_evt", stringListener)
	event.OnTyped[int](em, "type_evt", intListener)
	assert.Equal(t, 2, em.ListenersCount("type_evt"))

	// Remove only the string listener
	event.RemoveTypedListener[string](em, "type_evt", stringListener)
	assert.Equal(t, 1, em.ListenersCount("type_evt"))

	// Fire with int data - should only trigger int listener
	err, _ := event.FireTyped[int](em, "type_evt", 42)
	assert.NoError(t, err)
	assert.Equal(t, "int:42", buf.String())
}

func TestRemoveTypedListener_WithStdManagerAdapter(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	// Create a StdManagerAdapter explicitly
	adapter := event.StdForType[string]().(*event.StdManagerAdapter[string])

	// Register listener through adapter
	listener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("got:" + e.Data())
		return nil
	})
	adapter.On("adapter_evt", listener)
	assert.Equal(t, 1, adapter.ListenersCount("adapter_evt"))

	// Remove through adapter
	adapter.RemoveListener("adapter_evt", listener)
	assert.Equal(t, 0, adapter.ListenersCount("adapter_evt"))

	// Fire event - should not trigger
	err, _ := adapter.FireTyped("adapter_evt", "test")
	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRemoveTypedListener_WithOnceListener(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	// Register a Once listener
	listener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("got:" + e.Data())
		return nil
	})

	event.Once[string]("once_evt", listener)
	assert.Equal(t, 1, event.Std().ListenersCount("once_evt"))

	// Remove before firing
	event.RemoveTypedListener[string](event.Std(), "once_evt", listener)
	assert.Equal(t, 0, event.Std().ListenersCount("once_evt"))

	// Fire event - should not trigger
	err, _ := event.FireTyped[string](event.Std(), "once_evt", "test")
	assert.NoError(t, err)
	assert.Empty(t, buf.String())
}

func TestRemoveTypedListener_NonExistentListener(t *testing.T) {
	defer event.Reset()
	buf := new(bytes.Buffer)

	// Register a listener
	listener1 := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("listener1:" + e.Data())
		return nil
	})
	listener2 := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("listener2:" + e.Data())
		return nil
	})

	event.OnTyped[string](event.Std(), "nonexist_evt", listener1)
	assert.Equal(t, 1, event.Std().ListenersCount("nonexist_evt"))

	// Try to remove listener2 which was never added
	event.RemoveTypedListener[string](event.Std(), "nonexist_evt", listener2)
	assert.Equal(t, 1, event.Std().ListenersCount("nonexist_evt")) // count should remain same

	// Fire event - listener1 should still trigger
	err, _ := event.FireTyped[string](event.Std(), "nonexist_evt", "test")
	assert.NoError(t, err)
	assert.Equal(t, "listener1:test", buf.String())
}

func TestFireTyped(t *testing.T) {
	defer event.Reset()

	em := event.Std()
	buf := new(bytes.Buffer)

	// Register listener expecting string data
	listener := event.NewListenerFunc[string](func(e event.Event[string]) error {
		buf.WriteString("got:" + e.Data())
		return nil
	})

	event.OnTyped[string](em, "str_evt", listener)

	// FireTyped with matching type
	err, evt := event.FireTyped[string](em, "str_evt", "test data")
	assert.NoError(t, err)
	assert.Equal(t, "got:test data", buf.String())
	assert.Equal(t, "str_evt", evt.Name())
	buf.Reset()

	// FireTyped with mismatched type - should fail
	err2, evt2 := event.FireTyped[int](em, "str_evt", 123)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "data type mismatch")
	assert.Nil(t, evt2)
	assert.Empty(t, buf.String())
}
func TestOnce(t *testing.T) {
	defer event.Reset()

	event.Once[event.M]("evt1", event.NewListenerFunc[event.M](emptyListener))
	assert.True(t, event.Std().HasListeners("evt1"))
	err, _ := event.Trigger[event.M]("evt1", event.M{})
	if err != nil {
		t.Error(err)
	}
	assert.False(t, event.Std().HasListeners("evt1"))
}
