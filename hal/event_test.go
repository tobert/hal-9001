package hal

import (
	"fmt"
	"testing"
)

func TestEvtBodyAsArgv(t *testing.T) {
	evt := Evt{}
	evt.Body = "a simple flat test"
	argv := evt.BodyAsArgv()

	if len(argv) != 4 {
		fmt.Printf("expected 4 elements, got %d", len(argv))
		t.Fail()
	}

	//            1     2      3    4            5     6              7                    8
	evt.Body = ` !foo --this -one "is a little" more (complicated) 'becuase of the quotes' OK`
	argv = evt.BodyAsArgv()

	if len(argv) != 8 {
		fmt.Printf("expected 8 elements, got %d", len(argv))
		t.Fail()
	}

	// leading/trailing whitespace should be stripped and embedded quotes
	// should be intact. Escaped quotes are not supported.
	evt.Body = `	!bar "perhaps 'this challenge' will" '@%$*#@(**W(IOWIE-'------ break TEH BANK "" '' `
	argv = evt.BodyAsArgv()

	if len(argv) != 9 {
		fmt.Printf("expected 9 elements, got %d", len(argv))
		t.Fail()
	}
}
