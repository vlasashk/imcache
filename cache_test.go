package imcache

import (
	"math/rand"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestImcache_Get(t *testing.T) {
	tests := []struct {
		name string
		c    func() Imcache[string, string]
		key  string
		want string
		ok   bool
	}{
		{
			name: "success",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name: "not found",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
		},
		{
			name: "entry expired",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
		{
			name: "success - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "bar", WithNoExpiration())
				return c
			},
			key:  "foo",
			want: "bar",
			ok:   true,
		},
		{
			name: "not found - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				return c
			},
			key: "foo",
		},
		{
			name: "entry expired - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "bar", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			got, ok := c.Get(tt.key)
			if ok != tt.ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("Cache.Get(%s) = %v, _ want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestImcache_Get_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_Set(t *testing.T) {
	tests := []struct {
		name string
		c    func() Imcache[string, string]
		key  string
		val  string
	}{
		{
			name: "add new entry",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "replace existing entry - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			c.Set(tt.key, tt.val, WithNoExpiration())
			got, ok := c.Get(tt.key)
			if !ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, true", tt.key, ok)
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, want %v", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_GetOrSet(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Imcache[string, string]
		key     string
		val     string
		want    string
		present bool
	}{
		{
			name: "add new entry",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "add new entry if old expired",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "get existing entry",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			want:    "foo",
			present: true,
		},
		{
			name: "add new entry - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "add new entry if old expired - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key:  "foo",
			val:  "bar",
			want: "bar",
		},
		{
			name: "get existing entry - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			want:    "foo",
			present: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			got, ok := c.GetOrSet(tt.key, tt.val, WithDefaultExpiration())
			if ok != tt.present {
				t.Errorf("Cache.GetOrSet(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if got != tt.want {
				t.Errorf("Cache.GetOrSet(%s) = %v, _, want %v, _", tt.key, got, tt.want)
			}
		})
	}
}

func TestImcache_Replace(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Imcache[string, string]
		key     string
		val     string
		present bool
	}{
		{
			name: "success",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			present: true,
		},
		{
			name: "entry expired",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry doesn't exist",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "success - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			val:     "bar",
			present: true,
		},
		{
			name: "entry expired - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			val: "bar",
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			val: "bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.Replace(tt.key, tt.val, WithDefaultExpiration()); ok != tt.present {
				t.Fatalf("Cache.Replace(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
			}
			got, ok := c.Get(tt.key)
			if ok != tt.present {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if !ok {
				return
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, _, want %v, _", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_ReplaceWithFunc(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Imcache[string, int32]
		key     string
		f       func(int32) int32
		want    bool
		val     int32
		present bool
	}{
		{
			name: "success",
			c: func() Imcache[string, int32] {
				c := New[string, int32]()
				c.Set("foo", 997, WithNoExpiration())
				return c
			},
			key:     "foo",
			f:       Increment[int32],
			want:    true,
			val:     998,
			present: true,
		},
		{
			name: "entry expired",
			c: func() Imcache[string, int32] {
				c := New[string, int32]()
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "entry doesn't exist",
			c: func() Imcache[string, int32] {
				c := New[string, int32]()
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "success - sharded",
			c: func() Imcache[string, int32] {
				c := NewSharded[string, int32](2, DefaultStringHasher64{})
				c.Set("foo", 997, WithNoExpiration())
				return c
			},
			key:     "foo",
			f:       Decrement[int32],
			want:    true,
			val:     996,
			present: true,
		},
		{
			name: "entry expired - sharded",
			c: func() Imcache[string, int32] {
				c := NewSharded[string, int32](4, DefaultStringHasher64{})
				c.Set("foo", 997, WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() Imcache[string, int32] {
				c := NewSharded[string, int32](8, DefaultStringHasher64{})
				return c
			},
			key: "foo",
			f:   Increment[int32],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.ReplaceWithFunc(tt.key, tt.f, WithDefaultExpiration()); ok != tt.present {
				t.Fatalf("Cache.Replace(%s, _, _) = %t, want %t", tt.key, ok, tt.present)
			}
			got, ok := c.Get(tt.key)
			if ok != tt.present {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, %t", tt.key, ok, tt.present)
			}
			if got != tt.val {
				t.Errorf("Cache.Get(%s) = %v, _, want %v, _", tt.key, got, tt.val)
			}
		})
	}
}

func TestImcache_Remove(t *testing.T) {
	tests := []struct {
		name    string
		c       func() Imcache[string, string]
		key     string
		present bool
	}{
		{
			name: "success",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key:     "foo",
			present: true,
		},
		{
			name: "entry doesn't exist",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "bar",
		},
		{
			name: "entry expired",
			c: func() Imcache[string, string] {
				c := New[string, string]()
				c.Set("foo", "foo", WithExpiration(time.Nanosecond))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
		{
			name: "success - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](2, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpirationDate(time.Now().Add(time.Minute)))
				return c
			},
			key:     "foo",
			present: true,
		},
		{
			name: "entry doesn't exist - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](4, DefaultStringHasher64{})
				c.Set("foo", "foo", WithNoExpiration())
				return c
			},
			key: "bar",
		},
		{
			name: "entry expired - sharded",
			c: func() Imcache[string, string] {
				c := NewSharded[string, string](8, DefaultStringHasher64{})
				c.Set("foo", "foo", WithExpirationDate(time.Now().Add(time.Nanosecond)))
				<-time.After(time.Nanosecond)
				return c
			},
			key: "foo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c()
			if ok := c.Remove(tt.key); ok != tt.present {
				t.Fatalf("Cache.Remove(%s) = %t, want %t", tt.key, ok, tt.present)
			}
			if _, ok := c.Get(tt.key); ok {
				t.Fatalf("Cache.Get(%s) = _, %t, want _, false", tt.key, ok)
			}
		})
	}
}

func TestImcache_RemoveAll(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_RemoveStale(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			c.RemoveStale()
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			if _, ok := c.Get("bar"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "bar", ok)
			}
		})
	}
}

func TestImcache_GetAll(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](4, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithNoExpiration())
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			got := c.GetAll()
			want := map[string]string{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_GetAll_SlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New[string, string](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, string](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithSlidingExpiration(500*time.Millisecond))
			c.Set("bar", "bar", WithSlidingExpiration(500*time.Millisecond))
			<-time.After(300 * time.Millisecond)
			want := map[string]string{
				"foo": "foo",
				"bar": "bar",
			}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(300 * time.Millisecond)
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			<-time.After(500 * time.Millisecond)
			want = map[string]string{}
			if got := c.GetAll(); !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
		})
	}
}

func TestImcache_Len(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, int]
	}{
		{
			name: "not sharded",
			c:    New[string, int](),
		},
		{
			name: "sharded",
			c:    NewSharded[string, int](8, DefaultStringHasher64{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			n := 1000 + rand.Intn(1000)
			for i := 0; i < n; i++ {
				c.Set(strconv.Itoa(i), i, WithNoExpiration())
			}
			if got := c.Len(); got != n {
				t.Errorf("Cache.Len() = %d, want %d", got, n)
			}
		})
	}
}

func TestImcache_DefaultExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New(WithDefaultExpirationOption[string, string](500 * time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithDefaultExpirationOption[string, string](500*time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithDefaultExpiration())
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

func TestImcache_DefaultSlidingExpiration(t *testing.T) {
	tests := []struct {
		name string
		c    Imcache[string, string]
	}{
		{
			name: "not sharded",
			c:    New(WithDefaultSlidingExpirationOption[string, string](500 * time.Millisecond)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithDefaultSlidingExpirationOption[string, string](500*time.Millisecond)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.c
			c.Set("foo", "foo", WithDefaultExpiration())
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(300 * time.Millisecond)
			if _, ok := c.Get("foo"); !ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, true", "foo", ok)
			}
			<-time.After(500 * time.Millisecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
		})
	}
}

type evictionCallbackCall struct {
	key    string
	val    interface{}
	reason EvictionReason
}

type evictionCallbackMock struct {
	mu    sync.Mutex
	calls []evictionCallbackCall
}

func (m *evictionCallbackMock) Callback(key string, val interface{}, reason EvictionReason) {
	m.mu.Lock()
	m.calls = append(m.calls, evictionCallbackCall{key, val, reason})
	m.mu.Unlock()
}

func (m *evictionCallbackMock) HasBeenCalledWith(key string, val interface{}, reason EvictionReason) bool {
	m.mu.Lock()
	var call evictionCallbackCall
	for _, c := range m.calls {
		if c.key == key {
			call = c
			break
		}
	}
	m.mu.Unlock()
	return call.key == key && call.val == val && call.reason == reason
}

func (m *evictionCallbackMock) HasNotBeenCalled() bool {
	return len(m.calls) == 0
}

func (m *evictionCallbackMock) Reset() {
	m.calls = nil
}

func TestImcache_Get_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			if _, ok := c.Get("foo"); ok {
				t.Errorf("Cache.Get(%s) = _, %t, want _, false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_Set_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.Set("foo", "bar", WithNoExpiration())
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			c.Set("bar", "foo", WithNoExpiration())
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_GetOrSet_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("Cache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			<-time.After(time.Nanosecond)
			if _, ok := c.GetOrSet("foo", "foo", WithExpiration(time.Nanosecond)); ok {
				t.Errorf("Cache.GetOrSet(%s, _, _) = _, %t, want _, false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_Replace_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.Replace("foo", "bar", WithNoExpiration()); ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if ok := c.Replace("bar", "foo", WithNoExpiration()); !ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_ReplaceWithFunc_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", 1, WithExpiration(time.Nanosecond))
			c.Set("bar", 2, WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.ReplaceWithFunc("foo", func(interface{}) interface{} { return 997 }, WithNoExpiration()); ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", 1, EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "foo", 1, EvictionReasonExpired)
			}
			if ok := c.ReplaceWithFunc("bar", func(interface{}) interface{} { return 997 }, WithNoExpiration()); !ok {
				t.Errorf("Cache.Replace(%s, _, _) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", 2, EvictionReasonReplaced) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %d, %d)", "bar", 2, EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_Remove_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			if ok := c.Remove("foo"); ok {
				t.Errorf("Cache.Remove(%s) = %t, want false", "foo", ok)
			}
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if ok := c.Remove("bar"); !ok {
				t.Errorf("Cache.Remove(%s) = %t, want true", "bar", ok)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonReplaced)
			}
		})
	}
}

func TestImcache_RemoveAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveAll()
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonRemoved) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonRemoved)
			}
		})
	}
}

func TestImcache_RemoveStale_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := New(WithEvictionCallbackOption(evictioncMock.Callback))
			c.Set("foo", "foo", WithExpiration(time.Nanosecond))
			c.Set("bar", "bar", WithNoExpiration())
			<-time.After(time.Nanosecond)
			c.RemoveStale()
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_GetAll_EvictionCallback(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithNoExpiration())
			c.Set("foobar", "foobar", WithSlidingExpiration(time.Second))
			c.Set("bar", "bar", WithExpiration(time.Nanosecond))
			<-time.After(time.Nanosecond)
			got := c.GetAll()
			want := map[string]interface{}{
				"foo":    "foo",
				"foobar": "foobar",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Cache.GetAll() = %v, want %v", got, want)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonExpired)
			}
		})
	}
}

func TestNewSharded_NSmallerThan0(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(-1, _) did not panic")
		}
	}()
	NewSharded[string, string](-1, DefaultStringHasher64{})
}

func TestNewSharded_NilHasher(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("NewSharded(_, nil) did not panic")
		}
	}()
	_ = NewSharded[string, string](2, nil)
}

func TestImcache_StartCleaner(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			// StartCleaner should return an error if the interval is equal
			// or less than 0.
			if err := c.StartCleaner(0); err == nil {
				t.Fatalf("Cache.StartCleaner(_) = nil, want error")
			}
			if err := c.StartCleaner(-10); err == nil {
				t.Fatalf("Cache.StartCleaner(_) = nil, want error")
			}
			// Valid interval should start a cleaner.
			if err := c.StartCleaner(20 * time.Millisecond); err != nil {
				t.Fatalf("Cache.StartCleaner(_) = %v, want nil", err)
			}
			// Subsequent calls to StartCleaner should not start a new cleaner
			// and should return an error.
			if err := c.StartCleaner(5 * time.Nanosecond); err == nil {
				t.Fatalf("Cache.StartCleaner(_) = nil, want error")
			}
			if err := c.StartCleaner(7 * time.Nanosecond); err == nil {
				t.Fatalf("Cache.StartCleaner(_) = nil, want error")
			}
			<-time.After(30 * time.Millisecond)
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonExpired)
			}
			if evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback not called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
			<-time.After(200 * time.Millisecond)
			if !evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
		})
	}
}

func TestImcache_StopCleaner(t *testing.T) {
	evictioncMock := &evictionCallbackMock{}

	tests := []struct {
		name string
		c    Imcache[string, interface{}]
	}{
		{
			name: "not sharded",
			c:    New(WithEvictionCallbackOption(evictioncMock.Callback)),
		},
		{
			name: "sharded",
			c:    NewSharded[string](8, DefaultStringHasher64{}, WithEvictionCallbackOption(evictioncMock.Callback)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer evictioncMock.Reset()
			c := tt.c
			c.Set("foo", "foo", WithExpiration(time.Millisecond))
			c.Set("bar", "bar", WithExpiration(time.Millisecond))
			c.Set("foobar", "foobar", WithExpiration(100*time.Millisecond))
			if err := c.StartCleaner(20 * time.Millisecond); err != nil {
				t.Fatalf("Cache.StartCleaner(_) = %v, want nil", err)
			}
			<-time.After(30 * time.Millisecond)
			if !evictioncMock.HasBeenCalledWith("foo", "foo", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foo", "foo", EvictionReasonExpired)
			}
			if !evictioncMock.HasBeenCalledWith("bar", "bar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "bar", "bar", EvictionReasonExpired)
			}
			if evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback not called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
			c.StopCleaner()
			// Subsequent calls to StopCleaner should do nothing.
			c.StopCleaner()
			c.StopCleaner()
			<-time.After(200 * time.Millisecond)
			if evictioncMock.HasBeenCalledWith("foobar", "foobar", EvictionReasonExpired) {
				t.Errorf("want EvictionCallback called with EvictionCallback(%s, %s, %d)", "foobar", "foobar", EvictionReasonExpired)
			}
		})
	}
}

func TestCache_ZeroValue(t *testing.T) {
	var c Cache[string, string]
	c.Set("foo", "bar", WithNoExpiration())
	if v, ok := c.Get("foo"); !ok || v != "bar" {
		t.Errorf("want Cache.Get(_) = %s, true, got %s, %t", "bar", v, ok)
	}
}
