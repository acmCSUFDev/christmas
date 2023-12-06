package main

import (
	"image/color"
	"log"
	"time"

	"github.com/Jon-Bright/ledctl/pixarray"
)

const numLEDs = 200

const wormLength = 1
const wormSpeed = 100 * time.Millisecond

var colorOn = color.RGBA{255, 255, 255, 0}

func main() {
	strip, err := pixarray.NewWS281x(
		numLEDs,      // LEDs
		3,            // 3 bytes per pixel
		pixarray.BGR, // BGR channel order
		800000,       // 800 KHz
		10,           // DMA 10
		[]int{12},    // GPIO 12
	)
	if err != nil {
		log.Fatalln("failed to create pixarray:", err)
	}

	var tail int // worm tail position

	ticker := time.NewTicker(wormSpeed)
	defer ticker.Stop()

	for range ticker.C {
		// Turn off the tail LED bulb.
		strip.SetPixel(tail, pixarray.Pixel{})

		// Move the worm tail forward.
		tail = (tail + 1) % numLEDs

		// Turn on the head LED bulb.
		head := (tail + wormLength - 1) % numLEDs
		strip.SetPixel(head, rgbaToPixel(colorOn))

		must(strip.Write())

		if tail == 0 {
			// Wait a bit before restarting the worm so it's easier to find it
			// in the video.
			time.Sleep(1 * time.Second)
		}
	}
}

func must(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func rgbaToPixel(c color.RGBA) pixarray.Pixel {
	return pixarray.Pixel{
		R: int(c.R),
		G: int(c.G),
		B: int(c.B),
	}
}
