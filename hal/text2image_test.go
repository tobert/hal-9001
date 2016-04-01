package hal_test

import (
	"github.com/netflix/hal-9001/hal"
	"image/color"
	"testing"
)

func Testtext2image(t *testing.T) {
	def := color.RGBA{R: 1, G: 1, B: 1, A: 1}

	samples := map[string][4]uint32{
		"ffffff":   [4]uint32{255, 255, 255, 255},
		"ffffffff": [4]uint32{255, 255, 255, 255},
		"000000ff": [4]uint32{0, 0, 0, 255},
		"000000aa": [4]uint32{0, 0, 0, 170},
		"88888888": [4]uint32{136, 136, 136, 136},
		"888888":   [4]uint32{136, 136, 136, 255},
		"f79e10":   [4]uint32{247, 158, 16, 255},
		"f79e10ff": [4]uint32{247, 158, 16, 255},
	}

	fd := hal.FixedFont()

	for str, expected := range samples {
		result := fd.ParseColor(str, def)
		t.Logf("%q => %q\n", str, result)

		r, g, b, a := result.RGBA()

		if r != expected[0] {
			t.Fail()
		}
		if g != expected[1] {
			t.Fail()
		}
		if b != expected[2] {
			t.Fail()
		}
		if a != expected[3] {
			t.Fail()
		}
	}
}
