package hal_test

import (
	"testing"
	"time"

	"github.com/netflix/hal-9001/hal"
)

type Whatever struct {
	Field1 string
	Field2 int
	Field3 map[string]string
}

func TestTtlCache(t *testing.T) {
	w1 := Whatever{
		Field1: "testing",
		Field2: 9,
		Field3: map[string]string{"foo": "bar"},
	}

	cache := hal.Cache()
	cache.Set("whatever", &w1, time.Hour*24)

	w2 := Whatever{}
	ttl, err := cache.Get("whatever", &w2)
	if err != nil {
		panic(err)
	}

	if ttl == 0 {
		t.Error("ttl expired way too early")
		t.Fail()
	}

	if w2.Field2 != w1.Field2 {
		t.Error("Field2 doesn't match")
		t.Fail()
	}

	if w2.Field3["foo"] != "bar" {
		t.Error("Field3 doesn't match")
		t.Fail()
	}
}
