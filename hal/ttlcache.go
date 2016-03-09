package hal

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

type ttlCache struct {
	items map[string]interface{}
	times map[string]time.Time
	ttls  map[string]time.Duration
	mut   sync.Mutex // concurrent access
	init  sync.Once  // initialize the singleton once
}

var ttlcache ttlCache

func Cache() *ttlCache {
	ttlcache.init.Do(func() {
		ttlcache.items = make(map[string]interface{})
		ttlcache.times = make(map[string]time.Time)
		ttlcache.ttls = make(map[string]time.Duration)
	})

	return &ttlcache
}

// Get retreives a cached value and stores the result in the value pointed to by v.
// The time to live is returned and may be 0 to indicate the item is expired.
// e.g.
// policies := []EscalationPolicy{}
// err = hal.Cache().Set("pagerduty.escalation_policies", &policies, time.Hour)
// ttl, err := hal.Cache().Get("pagerduty.escalation_policies", &policies)
// if err != nil { panic(err) }
// if ttl == 0 { panic("stale cache!") }
func (cache *ttlCache) Get(key string, v interface{}) (time.Duration, error) {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	ttl := time.Duration(0)
	age := time.Now().Sub(cache.times[key])
	if age.Seconds() < cache.ttls[key].Seconds() {
		// not expired, compute the ttl
		ttlsecs := cache.ttls[key].Seconds() - age.Seconds()
		ttl = time.Duration(int(ttlsecs)) * time.Second
	}

	cached := cache.items[key]
	vtype := reflect.TypeOf(v)
	ctype := reflect.TypeOf(cached)

	// make sure the input type matches the type in the cache
	if vtype != ctype {
		msg := fmt.Sprintf("Type mismatch: got %q, expected %q", vtype.Name(), ctype.Name())
		return ttl, errors.New(msg)
	}

	// make sure it's a pointer and is not nil
	vval := reflect.ValueOf(v)
	if vval.Kind() != reflect.Ptr || vval.IsNil() {
		return ttl, errors.New("The second argument of Get() must be a non-nil pointer.")
	}

	// set the value
	cval := reflect.ValueOf(cached)
	vval.Elem().Set(cval.Elem())

	return ttl, nil
}

func (cache *ttlCache) Set(key string, v interface{}, ttl time.Duration) {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	cache.items[key] = v
	cache.times[key] = time.Now()
	cache.ttls[key] = ttl
}

func (cache *ttlCache) Delete(key string) {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	delete(cache.items, key)
	delete(cache.times, key)
	delete(cache.ttls, key)
}

func (cache *ttlCache) Age(key string) time.Duration {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	return time.Now().Sub(cache.times[key])
}

func (cache *ttlCache) Ttl(key string) time.Duration {
	cache.mut.Lock()
	defer cache.mut.Unlock()

	return cache.ttls[key]
}
