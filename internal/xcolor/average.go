package xcolor

import (
	"math"

	"dev.acmcsuf.com/christmas/internal/intmath"
)

// AveragingPoint is a point that is used to average a slice of colors into a
// single color.
type AveragingPoint struct {
	Color     RGB
	Intensity float32
}

type AveragingType uint8

const (
	_ AveragingType = iota // default
	SimpleAveragingType
	SquaredAveragingType
	NearestNeighborAveragingType
)

// AveragingFunc is a function that averages a slice of colors into a single
// color.
type AveragingFunc func([]AveragingPoint) RGB

// NewSimpleAveraging creates a new AveragingFunc that averages a slice of
// colors into a single color. It simply averages the red, green, and blue
// values of the colors.
//
// This function can handle about 2.8 million colors before overflowing.
func NewSimpleAveraging() AveragingFunc {
	const maxColors = math.MaxInt32 / (3 * 0xFF)
	return func(points []AveragingPoint) RGB {
		var r, g, b uint32
		for _, p := range points {
			r += uint32(p.Color.R)
			g += uint32(p.Color.G)
			b += uint32(p.Color.B)
		}
		n := uint32(len(points))
		return RGB{
			R: uint8(r / n),
			G: uint8(g / n),
			B: uint8(b / n),
		}
	}
}

// NewSquaredAveraging creates a new AveragingFunc that averages a slice of
// colors into a single color. It squares the red, green, and blue values of the
// colors before averaging them to give more weight to brighter colors.
//
// This function can handle about 33 thousand colors before overflowing.
//
// For more information, see:
// https://sighack.com/post/averaging-rgb-colors-the-right-way
func NewSquaredAveraging() AveragingFunc {
	const maxColors = math.MaxInt32 / (3 * 0xFF * 0xFF)
	return func(points []AveragingPoint) RGB {
		var r, g, b uint32
		for _, p := range points {
			r += uint32(p.Color.R) * uint32(p.Color.R)
			g += uint32(p.Color.G) * uint32(p.Color.G)
			b += uint32(p.Color.B) * uint32(p.Color.B)
		}
		n := uint32(len(points))
		return RGB{
			R: uint8(intmath.Sqrt32(int32(r / n))),
			G: uint8(intmath.Sqrt32(int32(g / n))),
			B: uint8(intmath.Sqrt32(int32(b / n))),
		}
	}
}

// NewNearestAveraging creates a new AveragingFunc that averages a slice of
// colors into a single color. It simply returns the first color in the slice.
func NewNearestAveraging() AveragingFunc {
	return func(points []AveragingPoint) RGB {
		if len(points) == 0 {
			return RGB{}
		}
		return points[0].Color
	}
}
