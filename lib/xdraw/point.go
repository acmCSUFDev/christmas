package xdraw

import "image"

// PtIx returns the flat index of the given point in the given rectangle.
func PtIx(r image.Rectangle, x, y int) int {
	return (y-r.Min.Y)*r.Dx() + (x - r.Min.X)
}
