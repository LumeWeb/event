package event_test

import (
	"fmt"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
	"go.lumeweb.com/event"
)

type testListener struct {
	userData string
}

func (l *testListener) Handle(e event.Event[event.M]) error {
	if ret := e.Data()["result"]; ret != nil {
		str := ret.(string) + fmt.Sprintf(" -> %s(%s)", e.Name(), l.userData)
		e.SetData(event.M{"result": str})
	} else {
		e.SetData(event.M{"result": fmt.Sprintf("handled: %s(%s)", e.Name(), l.userData)})
	}
	return nil
}

type testSubscriber struct {
	// ooo
}

func (s *testSubscriber) SubscribedEvents() map[string]any {
	return map[string]any{
		"e1": event.ListenerFunc[event.M](s.e1Handler),
		"e2": event.ListenerItem[event.M]{
			Priority: event.AboveNormal,
			Listener: event.ListenerFunc[event.M](func(e event.Event[event.M]) error {
				return fmt.Errorf("an error")
			}),
		},
		"e3": &testListener{},
	}
}

func (s *testSubscriber) e1Handler(e event.Event[event.M]) error {
	e.SetData(event.M{"e1-key": "val1"})
	return nil
}

type testSubscriber2 struct{}

func (s testSubscriber2) SubscribedEvents() map[string]any {
	return map[string]any{
		"e1": "invalid",
	}
}

type testEvent struct {
	name  string
	data  map[string]any
	abort bool
	props map[string]any
}

func (t *testEvent) Name() string {
	return t.name
}

func (t *testEvent) Data() map[string]any {
	return t.data
}

func (t *testEvent) SetData(m event.M) (event.Event[event.M], error) {
	t.data = m
	return t, nil
}

func (t *testEvent) Abort(b bool) {
	t.abort = b
}

func (t *testEvent) IsAborted() bool {
	return t.abort
}

func (t *testEvent) Get(key string) any {
	if t.props == nil {
		return nil
	}
	return t.props[key]
}

func (t *testEvent) Set(key string, val any) event.Event[event.M] {
	if t.props == nil {
		t.props = make(map[string]any)
	}
	t.props[key] = val
	return t
}

var _ event.Event[event.M] = &testEvent{}

func TestEvent(t *testing.T) {
	e := &event.BasicEvent[event.M]{}
	e.SetName("n1")
	data := event.M{
		"arg0": "val0",
	}
	e.SetData(data)
	e.SetTarget("tgt")

	newData := e.Data()
	newData["arg1"] = "val1"
	e.SetData(newData)

	assert.False(t, e.IsAborted())
	e.Abort(true)
	assert.True(t, e.IsAborted())

	assert.Equal(t, "n1", e.Name())
	assert.Equal(t, "tgt", e.Target())
	assert.Contains(t, e.Data(), "arg1")
	assert.Equal(t, "val0", e.Data()["arg0"])
	assert.Equal(t, nil, e.Data()["not-exist"])

	newData["arg1"] = "new val"
	e.SetData(newData)
	assert.Equal(t, "new val", e.Data()["arg1"])

	e1 := &event.BasicEvent[event.M]{}
	e1.SetName("e1")
	e1.SetData(event.M{"k": "v"})
	assert.Equal(t, "v", e1.Data()["k"])
	assert.NotEmpty(t, e1.Clone())
}
