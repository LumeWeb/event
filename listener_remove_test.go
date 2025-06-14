package event_test

import (
	"fmt"
	"testing"

	"github.com/gookit/goutil/testutil/assert"
	"go.lumeweb.com/event/v2"
)

type globalTestVal struct {
	n   int
	sum int
}
type testListenerCalc struct {
	bind  int
	owner *globalTestVal
	id    string
}

func (l *testListenerCalc) Handle(e event.Event[int]) error {
	l.owner.n++
	l.owner.sum += e.Data() + l.bind
	return nil
}

func (l *testListenerCalc) ID() string {
	if l.id == "" {
		l.id = fmt.Sprintf("testListenerCalc_%d", l.bind)
	}
	return l.id
}

// newTestListenerCalc creates a new testListenerCalc instance
func newTestListenerCalc(bind int, owner *globalTestVal) *testListenerCalc {
	return &testListenerCalc{
		bind:  bind,
		owner: owner,
	}
}

func Test_RemoveListener(t *testing.T) {
	t.Run("make func", func(t *testing.T) {
		global := &globalTestVal{}
		makeFn := func(a int) event.Listener[int] {
			return event.NewListenerFunc[int](func(e event.Event[int]) error {
				global.n++
				global.sum += e.Data() + a
				return nil
			})
		}

		evBus := event.NewManager[int]("")
		const evName = "ev1"

		f1 := makeFn(11)
		f2 := makeFn(22)
		f3 := makeFn(33)
		p4 := newTestListenerCalc(44, global)
		p5 := newTestListenerCalc(55, global)
		p6 := newTestListenerCalc(66, global)

		evBus.On(evName, f1)
		evBus.On(evName, f2)
		evBus.On(evName, f3)
		evBus.On(evName, p4)
		evBus.On(evName, p5)
		evBus.On(evName, p6)

		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 6)
		assert.Equal(t, global.sum, 231) // 11+22+33+44+55+66=231

		evBus.RemoveListener(evName, f2)
		evBus.RemoveListener(evName, p5)
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 6+4)
		assert.Equal(t, global.sum, 385) // 231+11+33+44+66=385

		evBus.RemoveListener(evName, f1)
		evBus.RemoveListener(evName, f1) // not exist function.
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 6+4+3)
		assert.Equal(t, global.sum, 528) // 385+33+44+66=528

		evBus.RemoveListener(evName, p6)
		evBus.RemoveListener(evName, p6) // not exist function.
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 6+4+3+2)
		assert.Equal(t, global.sum, 605) // 528+33+44=605
	})

	t.Run("same value struct", func(t *testing.T) {
		global := &globalTestVal{}
		f1 := newTestListenerCalc(11, global)
		f2 := newTestListenerCalc(22, global)
		f2same := newTestListenerCalc(22, global)
		f2copy := f2 // shares same instance

		evBus := event.NewManager[int]("")
		const evName = "ev1"
		evBus.On(evName, f1)
		evBus.On(evName, f2)
		evBus.On(evName, f2same)
		evBus.On(evName, f2copy)

		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 2)
		assert.Equal(t, global.sum, 33) // 11+22=33

		evBus.RemoveListener(evName, f1)
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 3)
		assert.Equal(t, global.sum, 55) // 33+22=55

		evBus.RemoveListener(evName, f2)
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 3)
		assert.Equal(t, global.sum, 55)
	})

	t.Run("same func", func(t *testing.T) {
		globalStatic = globalTestVal{} // reset before use
		global := &globalStatic

		f1 := event.NewListenerFunc[int](testFuncCalc1)
		f2 := event.NewListenerFunc[int](testFuncCalc2)
		f2same := event.NewListenerFunc[int](testFuncCalc2)
		f2copy := f2

		evBus := event.NewManager[int]("")
		const evName = "ev1"
		evBus.On(evName, f1)
		evBus.On(evName, f2)
		evBus.On(evName, f2same)
		evBus.On(evName, f2copy)

		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 3)
		assert.Equal(t, global.sum, 55) // 11+22+22=55

		evBus.RemoveListener(evName, f1)
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 5)
		assert.Equal(t, global.sum, 99) // 55+22+22=99

		evBus.RemoveListener(evName, f2)
		evBus.MustFire(evName, 0)
		assert.Equal(t, global.n, 6)
		assert.Equal(t, global.sum, 121) // 99+22=121
	})
}

var globalStatic = globalTestVal{}

func testFuncCalc1(e event.Event[int]) error {
	globalStatic.n++
	globalStatic.sum += e.Data() + 11
	return nil
}
func testFuncCalc2(e event.Event[int]) error {
	globalStatic.n++
	globalStatic.sum += e.Data() + 22
	return nil
}
