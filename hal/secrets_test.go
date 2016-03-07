package hal_test

import (
	"testing"

	"github.com/netflix/hal-9001/hal"
)

func TestSecretsBasic(t *testing.T) {
	secrets := hal.Secrets()

	// make sure it returns the empty value
	if secrets.Get("whatever") != "" {
		t.Fail()
	}

	if secrets.Exists("whatever") {
		t.Fail()
	}

	secrets.Put("whatever", "foo")

	if !secrets.Exists("whatever") {
		t.Fail()
	}

	if secrets.Get("whatever") != "foo" {
		t.Fail()
	}

	secrets.Delete("whatever")

	if secrets.Exists("whatever") {
		t.Fail()
	}
}
