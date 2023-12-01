package xdraw

import (
	"image"
	"image/color"
	"image/draw"
)

// DrawCircle draws a circle onto the given image.
func DrawCircle(img draw.Image, center image.Point, radius int, color color.Color) {
	for x := center.X - radius; x <= center.X+radius; x++ {
		for y := center.Y - radius; y <= center.Y+radius; y++ {
			if (x-center.X)*(x-center.X)+(y-center.Y)*(y-center.Y) <= radius*radius {
				img.Set(x, y, color)
			}
		}
	}
}

// EachCirclePx calls fn for each pixel within the circle defined by center and
// radius.
func EachCirclePx(center image.Point, radius int, fn func(image.Point) (stop bool)) {
	// Iterate from the top-left corner of the bounding box of the circle to the
	// bottom-right corner, and check if each point is within the circle.
	//
	// See this ASCII art for an example:
	//
	//    x---o--o---x
	//    |o        o|
	//    o          o
	//    o          o
	//    |o        o|
	//    x---o--o---x
	//

	// subtract 1 from the radius because we want to iterate over the pixels
	// *within* the circle, not the pixels on the edge of the circle.
	radius--

	x0 := center.X - radius
	x1 := center.X + radius
	y0 := center.Y - radius
	y1 := center.Y + radius
	rr := radius * radius

	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if (x-center.X)*(x-center.X)+(y-center.Y)*(y-center.Y) <= rr {
				if fn(image.Point{X: x, Y: y}) {
					return
				}
			}
		}
	}
}
