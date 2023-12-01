package main

import (
	"image/color"
	"log"
	"time"

	"github.com/Jon-Bright/ledctl/pixarray"
	"dev.acmcsuf.com/christmas/internal/intmath"
)

const wormSpeed = 200 * time.Millisecond

var ledOrder = []int{
	0, 1, 49, 2, 3, 4, 48, 46, 43, 5, 47, 44, 45, 6, 42, 7, 41, 8, 9, 40, 39,
	10, 38, 11, 12, 37, 36, 35, 13, 14, 34, 15, 33, 16, 32, 31, 17, 18, 30, 29,
	19, 20, 21, 28, 27, 22, 23, 26, 25, 24,
}

var maxLEDHeight = (func() int {
	var max int
	for _, i := range ledOrder {
		max = intmath.Max(max, i)
	}
	return max
})()

func main() {
	strip, err := pixarray.NewWS281x(
		len(ledOrder), // 50 LEDs
		3,             // 3 bytes per pixel
		pixarray.BGR,  // BGR channel order
		800000,        // 800 KHz
		10,            // DMA 10
		[]int{12},     // GPIO 12
	)
	if err != nil {
		log.Fatalln("failed to create pixarray:", err)
	}

	ticker := time.NewTicker(wormSpeed)
	defer ticker.Stop()

	var i int
	for range ticker.C {
		strip.SetPixel(ledOrder[i], pixarray.Pixel{})

		i = (i + 1) % len(ledOrder)
		strip.SetPixel(ledOrder[i], rgbaToPixel(ledColor(i)))

		if err := strip.Write(); err != nil {
			log.Println("failed to write:", err)
		}
	}
}

var transColors = []color.RGBA{
	{91, 206, 250, 255},
	{245, 169, 184, 255},
	{255, 255, 255, 255},
	{245, 169, 184, 255},
	{91, 206, 250, 255},
}

func ledColor(i int) color.RGBA {
	y := ledOrder[i]
	c := y * len(transColors) / maxLEDHeight
	return transColors[c]
}

func rgbaToPixel(c color.RGBA) pixarray.Pixel {
	return pixarray.Pixel{
		R: int(c.R),
		G: int(c.G),
		B: int(c.B),
	}
}
