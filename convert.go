package event

import (
	"fmt"
	"reflect"
)

// ConvertData converts data from type T to M with consistent handling
func ConvertData[T, M any](data T) (M, error) {
	var mData M

	switch d := any(data).(type) {
	case M:
		mData = d
	case nil:
		// Return zero value for M
		mData = *new(M)
	default:
		return mData, fmt.Errorf("event: cannot convert data from %T to %T", data, mData)
	}

	return mData, nil
}

// ConvertEvent converts an Event[T] to Event[M] using consistent conversion logic
func ConvertEvent[T, M any](e Event[T]) (Event[M], error) {
	mData, err := ConvertData[T, M](e.Data())
	if err != nil {
		return nil, fmt.Errorf("event: %w for event %s", err, e.Name())
	}

	// Create adapter that preserves all original event properties
	adapter := &eventTypeAdapter[T, M]{
		original: e,
		data:     mData,
	}
	return adapter, nil
}

// eventTypeAdapter preserves all original event properties while converting data type
type eventTypeAdapter[T, M any] struct {
	original Event[T]
	data     M
	props    map[string]any // stores converted properties
}

func (a *eventTypeAdapter[T, M]) Name() string { return a.original.Name() }
func (a *eventTypeAdapter[T, M]) Data() M      { return a.data }
func (a *eventTypeAdapter[T, M]) SetData(d M) (Event[M], error) {
	a.data = d
	var newData T
	// Handle map types specially - create new empty map instead of zero value
	rt := reflect.TypeOf(newData)
	if rt != nil && rt.Kind() == reflect.Map {
		newData = reflect.MakeMap(rt).Interface().(T)
	}
	if _, err := a.original.SetData(newData); err != nil {
		return a, fmt.Errorf("failed to update original event data: %w", err)
	}
	return a, nil
}
func (a *eventTypeAdapter[T, M]) Abort(val bool)  { a.original.Abort(val) }
func (a *eventTypeAdapter[T, M]) IsAborted() bool { return a.original.IsAborted() }
func (a *eventTypeAdapter[T, M]) Get(key string) any {
	if a.props != nil {
		if val, ok := a.props[key]; ok {
			return val
		}
	}
	return a.original.Get(key)
}
func (a *eventTypeAdapter[T, M]) Set(key string, val any) Event[M] {
	if a.props == nil {
		a.props = make(map[string]any)
	}
	a.props[key] = val
	a.original.Set(key, val) // Also set on original to maintain consistency
	return a
}

// ConvertListener converts a Listener[M] to a Listener[T] using ConvertEvent
func ConvertListener[M, T any](listener Listener[M]) Listener[T] {
	return NewListenerFunc[T](func(e Event[T]) error {
		mEvent, err := ConvertEvent[T, M](e)
		if err != nil {
			return err
		}
		return listener.Handle(mEvent)
	})
}
