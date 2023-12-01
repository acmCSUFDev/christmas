package leddraw

import (
	"fmt"
	"image"
	"math"

	"dev.acmcsuf.com/christmas/lib/intmath"
	"dev.acmcsuf.com/christmas/lib/xcolor"
)

// LEDCanvas is a canvas of LED points.
type LEDCanvas struct {
	leds    LEDStrip
	ledData []ledData
	ledRect image.Rectangle

	// pixelData is a precalculated array of pixels. A pixel is deemed relevant
	// if it is within the radius of any LED. The radius is a custom parameter.
	pixelMap   map[image.Point]pixelData
	canvasRect image.Rectangle

	opts LEDCanvasOpts
}

// ledData is the data of an LED. It is specifically crafted to be as small as
// possible.
type ledData struct {
	// neighborPixels is a list of pixels that are within the radius of the LED.
	neighborPixels []pointIntensity
	// neighborAverages is a list of colors of the pixels that are within the
	// radius of the LED. It has the threshold already applied to it.
	neighborAverages []xcolor.AveragingPoint
}

type pointIntensity struct {
	image.Point
	Intensity float64
}

type pixelData struct {
	neighborLEDs []neighborLED
}

type neighborLED struct {
	// index is the index of the nearest LED to the pixel.
	index int32
	// intensity is multiplied by the color of the pixel to get the color of the
	// LED. LEDs that have multiple pixels within their radius will have their
	// color averaged.
	//
	// TODO: instead of averaging, use the pixel that is closest to the LED.
	// Perhaps consider more complex algorithms, like Lanczos resampling.
	intensity float32
}

func applyIntensity(c xcolor.RGB, intensity float32) xcolor.RGB {
	return xcolor.RGB{
		R: uint8(float32(c.R) * intensity),
		G: uint8(float32(c.G) * intensity),
		B: uint8(float32(c.B) * intensity),
	}
}

// LEDCanvasOpts is a set of options for creating a new LEDCanvas.
type LEDCanvasOpts struct {
	// Intensity is the intensity function used to calculate the intensity of a
	// pixel based on the distance between the pixel and the nearest LED.
	Intensity IntensityFunc
	// Average is the averaging function used to average the colors of the
	// pixels that are within the radius of an LED.
	Average xcolor.AveragingFunc
	// PPI is the number of pixels per inch of the final LED canvas. The higher
	// the PPI, the higher the resolution of the final LED canvas.
	PPI float64
}

// NewLEDCanvas creates a new LEDCanvas from the given LED positions.
//
// ledPositions is a slice of points, where each point represents the position
// of an LED.
func NewLEDCanvas(ledPositions []image.Point, opts LEDCanvasOpts) (*LEDCanvas, error) {
	const maxLEDs = math.MaxInt32
	if len(ledPositions) > maxLEDs {
		return nil, fmt.Errorf("too many LEDs (%d), max %d", len(ledPositions), maxLEDs)
	}

	var ledRect image.Rectangle
	for _, ledPos := range ledPositions {
		ledRect.Min.X = intmath.Min(ledRect.Min.X, ledPos.X)
		ledRect.Min.Y = intmath.Min(ledRect.Min.Y, ledPos.Y)
		ledRect.Max.X = intmath.Max(ledRect.Max.X, ledPos.X)
		ledRect.Max.Y = intmath.Max(ledRect.Max.Y, ledPos.Y)
	}

	// Translate the LED positions so that the top left LED is at (0, 0).
	for i, ledPos := range ledPositions {
		ledPositions[i] = ledPos.Sub(ledRect.Min)
	}

	aspectRatio := float64(ledRect.Dx()) / float64(ledRect.Dy())
	canvasRect := image.Rect(0, 0, int(opts.PPI*aspectRatio), int(opts.PPI))

	if opts.PPI == 0 {
		opts.PPI = 128
	}
	if opts.Intensity == nil {
		minDist := FindMinDistance(ledPositions)
		canvasScale := float64(canvasRect.Dx()) / float64(ledRect.Dx())
		opts.Intensity = NewStepIntensity(minDist.Distance / 2 * canvasScale)
	}
	if opts.Average == nil {
		opts.Average = xcolor.NewSquaredAveraging()
	}

	pixelMap := make(map[image.Point]pixelData, int(opts.PPI)*len(ledPositions))
	leds := make([]ledData, len(ledPositions))

	for i, led := range ledPositions {
		nearestPixels := allPixelsWithIntensity(canvasRect, ledRect, led, opts.Intensity, 0.01)

		leds[i] = ledData{
			neighborPixels:   nearestPixels,
			neighborAverages: make([]xcolor.AveragingPoint, 0, len(nearestPixels)),
		}

		for _, pixel := range nearestPixels {
			data := pixelMap[pixel.Point]
			data.neighborLEDs = append(data.neighborLEDs, neighborLED{
				index:     int32(i),
				intensity: float32(pixel.Intensity),
			})
			pixelMap[pixel.Point] = data
		}
	}

	return &LEDCanvas{
		leds:       make(LEDStrip, len(ledPositions)),
		ledData:    leds,
		ledRect:    ledRect,
		pixelMap:   pixelMap,
		canvasRect: canvasRect,
		opts:       opts,
	}, nil
}

func ptIx(r image.Rectangle, x, y int) int {
	return (y-r.Min.Y)*r.Dx() + (x - r.Min.X)
}

// Bounds returns the bounds of the image canvas.
func (c *LEDCanvas) CanvasBounds() image.Rectangle {
	return c.canvasRect
}

// LEDBounds returns the boundary box of the LEDs on the LED canvas.
// The boundary box is the smallest rectangle that contains all LEDs.
func (c *LEDCanvas) LEDBounds() image.Rectangle {
	return c.ledRect
}

// Stride returns the stride of the LED canvas.
func (c *LEDCanvas) Stride() int {
	return c.canvasRect.Dx()
}

// LEDs returns the internal buffer of the LED canvas.
func (c *LEDCanvas) LEDs() LEDStrip {
	return c.leds
}

// Clear clears the LED canvas.
func (c *LEDCanvas) Clear() {
	c.leds.Clear()
}

// Render renders the given image to the LED canvas. The alpha channel of the
// image is ignored.
func (c *LEDCanvas) Render(src *image.RGBA) error {
	if !src.Rect.Eq(c.canvasRect) {
		return fmt.Errorf(
			"image bounds %v does not match canvas bounds %v",
			src.Rect, c.canvasRect)
	}

	c.Clear()

	// There are two main ways to render this image:
	//
	// 1. Iterate over all pixels in the image, and for each pixel, check if it
	//    has corresponding LEDs. If it does, calculate the intensity of the
	//    pixel and add it to the LED.
	// 2. Iterate over known pixels that have corresponding LEDs, and for each
	//    pixel, calculate the intensity of the pixel and add it to the LED.

	// TODO: benchmark
	c.render(src)

	return nil
}

func (c *LEDCanvas) render(src *image.RGBA) {
	y1 := src.Rect.Min.Y
	y2 := src.Rect.Max.Y
	x1 := src.Rect.Min.X
	x2 := src.Rect.Max.X

	for y := y1; y < y2; y++ {
		x := x1
		p := src.PixOffset(x, y)
		for x < x2 {
			data, ok := c.pixelMap[image.Point{X: x, Y: y}]
			if ok {
				color := xcolor.RGB{
					R: src.Pix[p+0],
					G: src.Pix[p+1],
					B: src.Pix[p+2],
				}

				// Calculate the intensity of the pixel.
				for _, led := range data.neighborLEDs {
					data := &c.ledData[led.index]
					data.neighborAverages = append(data.neighborAverages, xcolor.AveragingPoint{
						Color:     applyIntensity(color, led.intensity),
						Intensity: led.intensity,
					})
				}
			}

			x += 1
			p += 4
		}
	}

	// Reset the neighbor colors.
	for i, data := range c.ledData {
		if len(data.neighborAverages) == 0 {
			c.leds[i] = xcolor.RGB{}
		} else {
			c.leds[i] = c.opts.Average(data.neighborAverages)
		}
		// clear for next render
		c.ledData[i].neighborAverages = c.ledData[i].neighborAverages[:0]
	}
}

// allPixelsWithIntensity returns all pixels surrounding the given point that
// has an intensity greater than minIntensity.
func allPixelsWithIntensity(
	canvasRect, ledRect image.Rectangle,
	led image.Point,
	intensityFn IntensityFunc,
	minIntensity float64,
) []pointIntensity {
	// Calculate the scale between the LED and canvas.
	canvasScale := float64(canvasRect.Dx()) / float64(ledRect.Dx())

	// Calculate the pt nearest to the given LED point.
	// This is our "current position".
	pt := image.Point{
		X: int(float64(led.X-ledRect.Min.X) * canvasScale),
		Y: int(float64(led.Y-ledRect.Min.Y) * canvasScale),
	}

	// Save this point so we can calculate the distance.
	ledPt := pt

	// Iterate from the center point and outwards in a spiral.
	// See https://stackoverflow.com/a/3706260/5041327.

	direction := image.Point{X: 1, Y: 0}
	segmentLength := 1
	segmentPassed := 0

	// TODO: guesstimate the capacity
	points := make([]pointIntensity, 0, 0)

	// duds counts how many points have a lower intensity than minIntensity.
	var duds int

	for {
		// Scale the point back to the LED canvas.
		intensity := intensityFn(distance(pt, ledPt))
		if intensity > minIntensity {
			points = append(points, pointIntensity{
				Point:     pt,
				Intensity: intensity,
			})
		} else {
			duds++
		}

		pt = pt.Add(direction)
		segmentPassed++

		if segmentPassed == segmentLength {
			if duds == segmentLength {
				// The entire segment is duds, so we're done.
				// Going further out will not yield any more points.
				break
			}

			duds = 0
			segmentPassed = 0

			direction.X, direction.Y = -direction.Y, direction.X // rotate
			if direction.Y == 0 {
				segmentLength++
			}
		}
	}

	return points
}

func distance(pt1, pt2 image.Point) float64 {
	pt1x := float64(pt1.X)
	pt1y := float64(pt1.Y)
	pt2x := float64(pt2.X)
	pt2y := float64(pt2.Y)
	return math.Sqrt(math.Pow(pt1x-pt2x, 2) + math.Pow(pt1y-pt2y, 2))
}

// PtDistance is a pair of points and the distance between them.
type PtDistance struct {
	Pt1, Pt2 image.Point
	Distance float64
}

// FindMinDistance returns the pair of points with the smallest distance
// between them. It runs in O(n^2) time.
func FindMinDistance(points []image.Point) PtDistance {
	if len(points) < 2 {
		return PtDistance{}
	}

	minDistance := math.MaxFloat64
	var minPt1, minPt2 image.Point

	for i, pt1 := range points {
		for _, pt2 := range points[i+1:] {
			if d := distance(pt1, pt2); d < minDistance {
				minDistance = d
				minPt1 = pt1
				minPt2 = pt2
			}
		}
	}

	return PtDistance{
		Pt1:      minPt1,
		Pt2:      minPt2,
		Distance: minDistance,
	}
}
