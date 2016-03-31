package hal

import (
	"strings"
	"testing"
)

func TestUtf8Table(t *testing.T) {
	samples := [][][]string{
		{
			{"hdr"},
			{"one"},
		},
		{
			{"hdr"},
			{"one"},
			{"two"},
		},
		{
			{"left", "right"},
			{"one", "three"},
			{"two"},
		},
		{
			{"HEADER 1", "HDR 2", "LOL WUT"},
			{"one", "two", "three"},
			{"four", "five", "six"},
		},
		{
			{"Col 1", "Col 2", "3rd Column", "4th", "FIFTH"},
			{"one", "two", "three"},
			{"four", "five", "six"},
			{"hi"},
			{"", "", "", "-", "+"},
		},
	}

	var results [5]string

	results[0] = `
╔═════╗
║ hdr ║
╟─────╢
║ one ║
╚═════╝
`

	results[1] = `
╔═════╗
║ hdr ║
╟─────╢
║ one ║
║ two ║
╚═════╝
`

	results[2] = `
╔══════╤═══════╗
║ left │ right ║
╟──────┼───────╢
║  one │ three ║
║  two │       ║
╚══════╧═══════╝
`

	results[3] = `
╔══════════╤═══════╤═════════╗
║ HEADER 1 │ HDR 2 │ LOL WUT ║
╟──────────┼───────┼─────────╢
║      one │   two │   three ║
║     four │  five │     six ║
╚══════════╧═══════╧═════════╝
`

	results[4] = `
╔═══════╤═══════╤════════════╤═════╤═══════╗
║ Col 1 │ Col 2 │ 3rd Column │ 4th │ FIFTH ║
╟───────┼───────┼────────────┼─────┼───────╢
║   one │   two │      three │     │       ║
║  four │  five │        six │     │       ║
║    hi │       │            │     │       ║
║       │       │            │   - │     + ║
╚═══════╧═══════╧════════════╧═════╧═══════╝
`

	for i, sample := range samples {
		// first row is the header, the rest is data rows
		out := Utf8Table(sample[0], sample[1:])

		if len(out) == 0 {
			t.Fail()
		}

		trout := strings.TrimSpace(out)
		trres := strings.TrimSpace(results[i])

		if trout != trres {
			t.Logf("Got: \n%s\nExpected:\n%s\n", trout, trres)
			t.Fail()
		}
	}
}
