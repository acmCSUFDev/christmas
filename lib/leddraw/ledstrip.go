package leddraw

import (
	"image/color"

	"libdb.so/acm-christmas/internal/xcolor"
)

// LEDStrip is a strip of LEDs. It is represented as a slice of colors, where
// each color represents the color of an LED.
type LEDStrip []xcolor.RGB

// Setxcolor.RGBA sets the xcolor.RGBA color of the LED at index i. Alpha is ignored.
func (s LEDStrip) SetRGBA(i int, c color.RGBA) {
	s[i] = xcolor.RGBFromRGBA(c)
}

// Set sets the color of the LED at index i.
func (s LEDStrip) Set(i int, c color.Color) {
	s[i] = xcolor.RGBFromColor(c)
}

// Clear clears the LED strip.
func (s LEDStrip) Clear() {
	// This should be replaced with a memclr by the compiler.
	// On ARM, it does 32 bytes (~10 LEDs) at a time.
	for i := range s {
		s[i] = xcolor.RGB{}
	}
}
