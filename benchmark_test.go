package event_test

import (
	"testing"

	"go.lumeweb.com/event/v2"
)

func BenchmarkManager_Fire_no_listener(b *testing.B) {
	em := event.NewManager[any]("test")
	em.On("app.up", event.ListenerFunc[any](func(e event.Event[any]) error {
		return nil
	}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = em.Fire("app.up", nil)
	}
}

func BenchmarkManager_Fire_normal(b *testing.B) {
	em := event.NewManager[any]("test")
	em.On("app.up", event.ListenerFunc[any](func(e event.Event[any]) error {
		return nil
	}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = em.Fire("app.up", nil)
	}
}

func BenchmarkManager_Fire_wildcard(b *testing.B) {
	em := event.NewManager[any]("test")
	em.On("app.*", event.ListenerFunc[any](func(e event.Event[any]) error {
		return nil
	}))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = em.Fire("app.up", nil)
	}
}
