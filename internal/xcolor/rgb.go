package xcolor

import (
	"image"
	"image/color"
)

// NRGBAToRGBAImage converts an *image.NRGBA to an *image.RGBA.
// It does not perform alpha-premultiplication.
func NRGBAToRGBAImage(img *image.NRGBA) *image.RGBA {
	return (*image.RGBA)(img)
}

// RGB is a color in the RGB color space. It is represented as 3 8-bit values
// for red, green, and blue.
type RGB struct {
	R, G, B uint8
}

// RGBFromRGBA converts a color.RGBA to RGB.
func RGBFromRGBA(c color.RGBA) RGB {
	return RGB{c.R, c.G, c.B}
}

// RGBFromColor converts any color.Color to RGB.
func RGBFromColor(c color.Color) RGB {
	if c, ok := c.(color.RGBA); ok {
		return RGBFromRGBA(c)
	}
	r, g, b, _ := c.RGBA()
	return RGB{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
	}
}

// RGBFromUint converts an integer to RGB.
func RGBFromUint(u uint32) RGB {
	return RGB{
		R: uint8(u >> 16),
		G: uint8(u >> 8),
		B: uint8(u),
	}
}

// RGBA implements the color.Color interface.
func (c RGB) RGBA() (r, g, b, a uint32) {
	r = uint32(c.R)
	r |= r << 8
	g = uint32(c.G)
	g |= g << 8
	b = uint32(c.B)
	b |= b << 8
	a = 0xFFFF
	return
}

// ToUint converts the RGB color to an integer.
func (c RGB) ToUint() uint32 {
	return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}
