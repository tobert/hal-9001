package hal

import (
	"fmt"
	"testing"
)

func TestAsciiTable(t *testing.T) {
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

	for _, sample := range samples {
		// first row is the header, the rest is data rows
		out := AsciiTable(sample[0], sample[1:])
		// not a very useful test ... yet
		if len(out) == 0 {
			t.Fail()
		}

		fmt.Println(out)
	}
}
