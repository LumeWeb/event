package event_test

import (
	"testing"

	"github.com/gookit/goutil/testutil/assert"
	"go.lumeweb.com/event/v2"
)

func TestConvertData(t *testing.T) {
	t.Run("M to M", func(t *testing.T) {
		data := event.M{"key": "value"}
		converted, err := event.ConvertData[event.M, event.M](data)
		assert.NoError(t, err)
		assert.Equal(t, data, converted)
	})

	t.Run("nil to M", func(t *testing.T) {
		var data map[string]interface{}
		converted, err := event.ConvertData[any, event.M](data)
		assert.NoError(t, err)
		assert.Equal(t, event.M(nil), converted)
	})

	t.Run("string to M", func(t *testing.T) {
		data := "string data"
		_, err := event.ConvertData[string, event.M](data)
		assert.Error(t, err)
	})
}

func TestConvertEvent(t *testing.T) {
	t.Run("Event[M] to Event[M]", func(t *testing.T) {
		e := event.NewBasic("test", event.M{"key": "value"})
		converted, err := event.ConvertEvent[event.M, event.M](e)
		assert.NoError(t, err)
		assert.Equal(t, e.Name(), converted.Name())
		assert.Equal(t, e.Data(), converted.Data())
	})

	t.Run("Event[string] to Event[M]", func(t *testing.T) {
		e := event.NewBasic("test", "string data")
		_, err := event.ConvertEvent[string, event.M](e)
		assert.Error(t, err)
	})
}

func TestConvertListener(t *testing.T) {
	t.Run("Listener[string] to Listener[M]", func(t *testing.T) {
		listenerM := event.NewListenerFunc[event.M](func(e event.Event[event.M]) error {
			return nil
		})
		listenerString := event.ConvertListener[event.M, string](listenerM)

		e := event.NewBasic("test", "test data")
		err := listenerString.Handle(e)
		assert.Error(t, err)
	})
}

func TestEventTypeAdapter(t *testing.T) {
	t.Run("preserves all event properties", func(t *testing.T) {
		// Create original event with all properties set
		orig := event.NewBasic("test", event.M{"key": "value"})
		orig.Set("key1", "value1")
		orig.Abort(true)

		// Convert to adapter with same data type (event.M to event.M)
		adapter, err := event.ConvertEvent[event.M, event.M](orig)
		assert.NoError(t, err)

		// Verify all properties preserved
		assert.Equal(t, "test", adapter.Name())
		assert.Equal(t, event.M{"key": "value", "key1": "value1"}, adapter.Data())
		assert.Equal(t, "value", adapter.Data()["key"])
		assert.Equal(t, "value1", adapter.Data()["key1"])
		assert.True(t, adapter.IsAborted())
		assert.Equal(t, "value1", orig.Get("key1")) // verify original has property

		// Test setting data
		newData := event.M{"new": "data"}
		_, err = adapter.SetData(newData)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, newData, adapter.Data())

		// Test setting properties
		adapter.Set("key2", "value2")
		assert.Equal(t, "value2", adapter.Get("key2"))
		assert.Equal(t, "value2", orig.Get("key2")) // verify original updated

		// Test abort
		adapter.Abort(false)
		assert.False(t, adapter.IsAborted())
		assert.False(t, orig.IsAborted()) // verify original updated
	})

	t.Run("handles nil original data", func(t *testing.T) {
		orig := event.NewBasic[string]("test", "")
		adapter, err := event.ConvertEvent[string, string](orig)
		assert.NoError(t, err)
		assert.Equal(t, "", adapter.Data()) // should get zero value
	})
}
