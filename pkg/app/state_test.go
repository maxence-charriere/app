package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObserver(t *testing.T) {
	elem := &hello{}
	c := NewClientTester(elem)
	defer c.Close()

	isSubscribeCalled := false
	isObserving := true
	v := 42

	o := newObserver(elem, func(o *observer) {
		isSubscribeCalled = true
	})
	o.While(func() bool { return isObserving }).
		Value(&v)

	require.True(t, isSubscribeCalled)
	require.Equal(t, elem, o.element)
	require.Len(t, o.conditions, 1)
	require.NotNil(t, o.receiver)
	require.True(t, o.isObserving())

	c.Mount(Div())
	c.Consume()
	require.False(t, o.isObserving())

	c.Mount(elem)
	c.Consume()
	require.True(t, o.isObserving())

	isObserving = false
	require.False(t, o.isObserving())

	require.Panics(t, func() {
		var s string
		newObserver(elem, func(*observer) {}).Value(s)
	})
}

func TestStateIsExpired(t *testing.T) {
	utests := []struct {
		scenario  string
		state     State
		isExpired bool
	}{
		{
			scenario:  "state without expiration",
			state:     State{},
			isExpired: false,
		},
		{
			scenario:  "state is not expired",
			state:     State{ExpiresAt: time.Now().Add(time.Minute)},
			isExpired: false,
		},
		{
			scenario:  "state is expired",
			state:     State{ExpiresAt: time.Now().Add(-time.Minute)},
			isExpired: true,
		},
	}

	for _, u := range utests {
		t.Run(u.scenario, func(t *testing.T) {
			require.Equal(t, u.isExpired, u.state.isExpired(time.Now()))
		})
	}
}

func TestStore(t *testing.T) {
	d := NewClientTester(Div())
	defer d.Close()

	s := makeStore(d)
	defer s.Cleanup()
	key := "/test/store"

	var v int
	s.Get(key, &v)
	require.Zero(t, v)

	s.Set(key, 42)
	s.Get(key, &v)
	require.Equal(t, 42, v)

	s.Set(key, "21")
	s.Get(key, &v)
	require.Equal(t, 42, v)

	s.Del(key)
	require.Empty(t, s.states)
}

func TestStorePersist(t *testing.T) {
	d := NewClientTester(Div())
	defer d.Close()

	s := makeStore(d)
	key := "/test/store/persist"

	t.Run("value is pesisted", func(t *testing.T) {
		var v int

		s.Set(key, 42, Persist)
		s.Get(key, &v)
		require.Equal(t, 42, v)
		require.Equal(t, 1, d.localStorage().Len())
	})

	t.Run("value is not pesisted", func(t *testing.T) {
		var v int

		s.Set(key, struct {
			Func func()
		}{}, Persist)
		s.Get(key, &v)
		require.Equal(t, 0, v)
	})

	t.Run("value is obtained from local storage", func(t *testing.T) {
		var v int

		s.Set(key, 21, Persist)
		delete(s.states, key)
		require.Empty(t, s.states)

		s.Get(key, &v)
		require.Equal(t, 21, v)
		require.Equal(t, 1, d.localStorage().Len())
	})

	t.Run("value is observed from local storage", func(t *testing.T) {
		var v int

		s.Set(key, 84, Persist)
		delete(s.states, key)
		require.Empty(t, s.states)

		s.Observe(key, Div()).Value(&v)
		require.Equal(t, 84, v)
		require.Equal(t, 1, d.localStorage().Len())
	})

	t.Run("value is deleted", func(t *testing.T) {
		var v int

		s.Set(key, 1977, Persist)
		s.Del(key)

		require.Empty(t, s.states)
		s.Get(key, &v)
		require.Equal(t, 0, v)
		require.Equal(t, 0, d.localStorage().Len())
	})
}

func TestStoreEncrypt(t *testing.T) {
	d := NewClientTester(Div())
	defer d.Close()

	s := makeStore(d)
	key := "/test/store/crypt"

	t.Run("value is encrypted and decrypted", func(t *testing.T) {
		var v int

		s.Set(key, 42, Persist, Encrypt)
		s.Get(key, &v)
		require.Equal(t, 42, v)
		require.Equal(t, 2, d.localStorage().Len(), d.localStorage()) // Contain app ID.
	})

	t.Run("value is decrypted from local storage", func(t *testing.T) {
		var v int

		s.Set(key, 43, Persist)
		delete(s.states, key)

		s.Get(key, &v)
		require.Equal(t, 43, v)
		require.Equal(t, 2, d.localStorage().Len()) // Contain app ID.
	})
}

func TestStoreExpiresIn(t *testing.T) {
	d := NewClientTester(Div())
	defer d.Close()

	s := makeStore(d)
	key := "/test/store/expiresIn"

	t.Run("value is not expired", func(t *testing.T) {
		var v int

		s.Set(key, 42, Persist, ExpiresIn(time.Minute))
		s.Get(key, &v)
		require.Equal(t, 42, v)
		require.Equal(t, 1, d.localStorage().Len())
	})

	t.Run("get expired value", func(t *testing.T) {
		var v int

		s.Set(key, 21, Persist, ExpiresIn(-time.Minute))
		s.Get(key, &v)
		require.Equal(t, 0, v)
		require.Equal(t, 0, d.localStorage().Len())
	})

	t.Run("get persisted expired value", func(t *testing.T) {
		var v int

		s.Del(key)
		delete(s.states, key)

		s.disp.localStorage().Set(key, persistentState{
			ExpiresAt: time.Now().Add(-time.Minute),
		})

		s.Get(key, &v)
		require.Equal(t, 0, v)
		require.Equal(t, 0, d.localStorage().Len())
	})

	t.Run("observe expired value", func(t *testing.T) {
		var v int

		s.Set(key, 21, Persist, ExpiresIn(-time.Minute))
		s.Observe(key, Div()).Value(&v)
		require.Equal(t, 0, v)
		require.Equal(t, 0, d.localStorage().Len())
	})

	t.Run("expire expired values", func(t *testing.T) {
		s.Set(key, 99, Persist, ExpiresIn(time.Minute))
		require.Len(t, s.states, 1)
		require.Equal(t, 1, d.localStorage().Len())

		state := s.states[key]
		state.ExpiresAt = time.Now().Add(-time.Minute)
		s.states[key] = state
		require.True(t, state.isExpired(time.Now()))
		require.Equal(t, 1, d.localStorage().Len())

		s.expireExpiredValues()
		require.True(t, state.isExpired(time.Now()))
		require.Equal(t, 0, d.localStorage().Len())
	})
}

func TestStoreObserve(t *testing.T) {
	source := &foo{}
	d := NewClientTester(source)
	defer d.Close()

	s := makeStore(d)
	key := "/test/observe"

	s.Observe(key, source).Value(&source.Bar)
	require.Equal(t, "", source.Bar)
	require.Len(t, s.states, 1)
	require.Len(t, s.states[key].observers, 1)

	s.Set(key, "hello")
	d.Consume()
	require.Equal(t, "hello", source.Bar)

	s.Set(key, nil)
	d.Consume()
	require.Equal(t, "", source.Bar)

	s.Set(key, 42)
	d.Consume()
	require.Equal(t, "", source.Bar)

	d.Mount(Div())
	d.Consume()
	s.Set(key, "hi")
	require.Empty(t, s.states[key].observers)

	s.Set(key, 42)
	s.Observe(key, source).Value(&source.Bar)
	require.Equal(t, "", source.Bar)
}

func TestRemoveUnusedObservers(t *testing.T) {
	source := &foo{}
	d := NewClientTester(source)
	defer d.Close()

	s := makeStore(d)
	key := "/test/observe/remove"

	var v int
	n := 5
	for i := 0; i < 5; i++ {
		s.Observe(key, source).
			While(func() bool { return false }).
			Value(&v)
	}
	state := s.states[key]
	require.Len(t, state.observers, n)

	s.removeUnusedObservers()
	require.Empty(t, state.observers)
}

func TestStoreValue(t *testing.T) {
	nb := 42
	c := copyTester{pointer: &nb}

	utests := []struct {
		scenario string
		src      interface{}
		recv     interface{}
		expected interface{}
		err      bool
	}{
		{
			scenario: "value to exported field receiver",
			src:      42,
			recv:     &c.Exported,
			expected: 42,
		},
		{
			scenario: "value unexported field receiver",
			src:      21,
			recv:     &c.unexported,
			expected: 21,
		},
		{
			scenario: "nil to receiver",
			src:      nil,
			recv:     &c.unexported,
			expected: 0,
		},
		{
			scenario: "pointer to receiver",
			src:      new(int),
			recv:     &c.unexported,
			expected: 0,
		},
		{
			scenario: "nil to pointer receiver",
			src:      nil,
			recv:     &c.pointer,
			expected: (*int)(nil),
		},
		{
			scenario: "slice to receiver",
			src:      []int{14, 2, 86},
			recv:     &c.slice,
			expected: []int{14, 2, 86},
		},
		{
			scenario: "map to receiver",
			src:      map[string]int{"foo": 42},
			recv:     &c.mapp,
			expected: map[string]int{"foo": 42},
		},
		{
			scenario: "receiver have a different type",
			src:      "hello",
			recv:     &c.Exported,
			err:      true,
		},
		{
			scenario: "receiver is not a pointer",
			src:      51,
			recv:     c.Exported,
			err:      true,
		},
	}

	for _, u := range utests {
		t.Run(u.scenario, func(t *testing.T) {
			err := storeValue(u.recv, u.src)
			if u.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			recv := reflect.ValueOf(u.recv).Elem().Interface()
			require.Equal(t, u.expected, recv)
		})
	}
}

type copyTester struct {
	Exported   int
	unexported int
	pointer    *int
	slice      []int
	mapp       map[string]int
}
