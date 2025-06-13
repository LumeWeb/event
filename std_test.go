package event_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/gookit/goutil/testutil/assert"
	"go.lumeweb.com/event"
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

	event.On[event.M]("evt1", event.ListenerFunc[event.M](fn), 0)
	event.On[event.M]("evt1", event.ListenerFunc[event.M](emptyListener), event.High)
	assert.True(t, event.HasListeners("evt1"))

	err, e := event.Fire[event.M]("evt1", event.M{})
	assert.NoError(t, err)
	assert.Equal(t, "evt1", e.Name())
	assert.Equal(t, "event: evt1", buf.String())

	event.NewBasic("evt2", event.M{}).AttachTo(&event.StdManagerAdapter[event.M]{Mgr: event.Std()})
	event.On[event.M]("evt2", event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
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

	evt1 := event.NewBasic("evt1", event.M{"n": "inhere"})
	event.AddEvent[event.M](evt1)

	assert.True(t, event.HasEvent("evt1"))
	assert.False(t, event.HasEvent("not-exist"))

	event.Listen[event.M]("evt1", event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
		_, _ = fmt.Fprintf(buf, "event: %s, params: n=%s", e.Name(), e.Data()["n"])
		return nil
	}), event.Normal)

	assert.True(t, event.HasListeners("evt1"))
	assert.False(t, event.HasListeners("not-exist"))

	err := event.FireEvent[event.M](evt1)
	assert.NoError(t, err)
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
	buf.Reset()

	err = event.TriggerEvent[event.M](evt1)
	assert.NoError(t, err)
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
	buf.Reset()

	event.AsyncFire[event.M](evt1)
	time.Sleep(time.Second)
	assert.Equal(t, "event: evt1, params: n=inhere", buf.String())
}

func TestAsync(t *testing.T) {
	event.Reset()
	event.Config(event.UsePathMode)

	buf := new(bytes.Buffer)
	event.On[event.M]("test", event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
		buf.WriteString("test:")
		buf.WriteString(fmt.Sprintf("%v", e.Data()["key"]))
		buf.WriteString("|")
		return nil
	}))

	event.Async("test", event.M{"key": "val1"})
	te := &testEvent{name: "test", data: event.M{"key": "val2"}}
	event.FireAsync[event.M](te)

	err := event.CloseWait()
	if err != nil {
		t.Errorf("CloseWait() returned error: %v", err)
	}
	s := buf.String()
	assert.Contains(t, s, "test:val1|")
	assert.Contains(t, s, "test:val2|")
}

func TestFire_point_at_end(t *testing.T) {
	// clear all
	event.Reset()
	event.Listen[event.M]("db.*", event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
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

	event.On[event.M]("n1", event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
		return fmt.Errorf("an error")
	}), event.Max)
	event.On[event.M]("n2", event.ListenerFunc[event.M](emptyListener), event.Min)

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
		event.On[event.M]("", event.ListenerFunc[event.M](emptyListener), 0)
	})
	assert.Panics(t, func() {
		event.On[event.M]("name", nil, 0)
	})
	assert.Panics(t, func() {
		event.On[event.M]("++df", event.ListenerFunc[event.M](emptyListener), 0)
	})

	std := event.Std()
	event.On[event.M]("n1", event.ListenerFunc[event.M](emptyListener), event.Min)
	assert.Equal(t, 1, std.ListenersCount("n1"))
	assert.Equal(t, 0, std.ListenersCount("not-exist"))
	assert.True(t, event.HasListeners("n1"))
	assert.False(t, event.HasListeners("name"))

	assert.NotEmpty(t, std.Listeners())
	assert.NotEmpty(t, std.ListenersByName("n1"))

	std.RemoveListeners("n1")
	assert.False(t, event.HasListeners("n1"))
}
func TestOnce(t *testing.T) {
	defer event.Reset()

	event.Once[event.M]("evt1", event.ListenerFunc[event.M](emptyListener))
	assert.True(t, event.Std().HasListeners("evt1"))
	err, _ := event.Trigger[event.M]("evt1", event.M{})
	if err != nil {
		t.Error(err)
	}
	assert.False(t, event.Std().HasListeners("evt1"))
}
