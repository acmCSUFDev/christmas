package xdraw

import (
	"image"
	"image/draw"
)

type subImager interface {
	draw.Image
	SubImage(r image.Rectangle) image.Image
}

// SubImage returns an image representing the portion of the image p visible
// through r. The returned value may or may not share pixels with the original
// image.
func SubImage(img image.Image, bounds image.Rectangle) image.Image {
	subImage, ok := img.(subImager)
	if ok {
		return subImage.SubImage(bounds)
	}
	subImage = image.NewRGBA(img.Bounds())
	draw.Draw(subImage, subImage.Bounds(), img, image.Point{}, draw.Src)
	return subImage
}

// BoundingBox returns the smallest rectangle that contains all of the given
// points.
func BoundingBox(pts []image.Point) image.Rectangle {
	var box image.Rectangle
	for _, pt := range pts {
		r := image.Rectangle{
			Min: pt,
			Max: pt.Add(image.Point{X: 1, Y: 1}),
		}

		if box.Empty() {
			box = r
		} else {
			box = box.Union(r)
		}
	}
	return box
}
