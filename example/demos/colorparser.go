package main

// go run utf8table.go

import (
	"fmt"
	"github.com/netflix/hal-9001/hal"
	"image/color"
)

func main() {
	samples := []string{
		"ffffff",
		"ffffffff",
		"000000ff",
		"000000aa",
		"888888ff",
		"888888",
		"f79e10",   // amber
		"f79e10ff", // amber with alpha
	}

	fd := hal.FixedFont()

	for _, sample := range samples {
		result := fd.ParseColor(sample, color.Black)
		fmt.Printf("%q => %q\n", sample, result)
	}
}
