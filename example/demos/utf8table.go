package main

// go run utf8table.go

import (
	"fmt"
	"github.com/netflix/hal-9001/hal"
)

func main() {
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
		out := hal.Utf8Table(sample[0], sample[1:])
		fmt.Println(out)
	}
}
