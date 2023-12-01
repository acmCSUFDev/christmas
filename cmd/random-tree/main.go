package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	_ "embed"
	_ "image/jpeg"

	"dev.acmcsuf.com/christmas/lib/csvutil"
	"dev.acmcsuf.com/christmas/lib/intmath"
	"dev.acmcsuf.com/christmas/lib/vision"
	"dev.acmcsuf.com/christmas/lib/xdraw"
	"github.com/fogleman/poissondisc"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"

	_ "golang.org/x/image/bmp"
)

//go:embed description.txt
var description string

var (
	seed    = int64(0)
	numLED  = 50
	ledSize = 5
	outDir  = "."
	csvName = "led-points.csv"
	pngName = "led-points.png"
)

func init() {
	pflag.IntVarP(&numLED, "num-led", "n", numLED, "number of LEDs")
	pflag.IntVarP(&ledSize, "led-size", "l", ledSize, "LED size/radius in px")
	pflag.Int64VarP(&seed, "seed", "s", seed, "random seed")
	pflag.StringVarP(&outDir, "out-dir", "o", outDir, "output directory")
	pflag.StringVar(&csvName, "csv-name", csvName, "LED points CSV file name")
	pflag.StringVar(&pngName, "png-name", pngName, "LED points PNG image name")
}

func main() {
	log.SetFlags(0)

	pflag.Usage = func() {
		log.Println(description)
		log.Println("Usage:")
		log.Println("  random-tree [flags...] <mask-image>")
		log.Println()
		log.Println("Flags:")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type SubImage interface {
	draw.Image
	SubImage(r image.Rectangle) image.Image
}

func run() error {
	maskImageAny, err := decodeImageFile(pflag.Arg(0))
	if err != nil {
		return errors.Wrap(err, "failed to decode mask image")
	}

	maskImage, ok := maskImageAny.(SubImage)
	if !ok {
		maskImage = image.NewRGBA(maskImageAny.Bounds())
		draw.Draw(maskImage, maskImage.Bounds(), maskImageAny, image.ZP, draw.Src)
	}

	boundaryImage := vision.NewBoundaryImage(maskImage, color.White)

	boundarySize := boundaryImage.Count()
	if boundarySize < numLED {
		return fmt.Errorf(
			"boundary size (%d) is less than number of LEDs (%d)",
			boundarySize, numLED)
	}

	// Find the boundary box of the mask image, which is the smallest rectangle
	// that contains all the white pixels in the mask image.
	boundaryBox := boundaryImage.BoundaryBox()
	log.Println("boundary box:", boundaryBox)

	// Re-create the boundary image with the boundary box as the bounds.
	maskImage = maskImage.SubImage(boundaryBox).(SubImage)
	boundaryImage = vision.NewBoundaryImage(maskImage, color.White)

	rand := rand.New(rand.NewSource(seed))

	// Create an image for the Poisson Disk sampling. This image will have the
	// same size as the mask image.
	poissonImage := &image.Paletted{
		Pix:    make([]uint8, maskImage.Bounds().Dx()*maskImage.Bounds().Dy()),
		Stride: maskImage.Bounds().Dx(),
		Rect:   maskImage.Bounds(),
		Palette: []color.Color{
			color.Black,
			color.White,
		},
	}

	// Binary search for the right Poisson Disk parameter that would yield us
	// the exact number of points we need.
	//
	// We'll start by setting the maximum distance between points to the size of
	// the image. This will guarantee that we'll get at least one point. We will
	// then binary search until we get the exact number of points we need. The
	// smallest the distance should be is the size of the LED.
	//
	// The larger the distance, the less points we'll get. The smaller the
	// distance, the more points we'll get. This effectively means the searching
	// array is sorted in descending order.
	poissonRadius := searchFloat(
		0.01,
		float64(intmath.Max(maskImage.Bounds().Dx(), maskImage.Bounds().Dy())),
		0.01, // search the radius to the nearest 0.01
		func(r float64) bool {
			k := 10000 // max attempts to add neighbor points

			// Sample the Poisson Disk and draw the poissonPoints on the image.
			poissonPoints := poissondisc.Sample(
				float64(poissonImage.Bounds().Min.X),
				float64(poissonImage.Bounds().Min.Y),
				float64(poissonImage.Bounds().Max.X),
				float64(poissonImage.Bounds().Max.Y),
				r, k, rand)

			for _, ppt := range poissonPoints {
				pt := image.Pt(int(math.Round(ppt.X)), int(math.Round(ppt.Y)))
				poissonImage.SetColorIndex(pt.X, pt.Y, 1)
			}

			// Count the number of points on the boundary image.
			var count int
			boundaryImage.EachPt(func(pt image.Point) bool {
				if poissonImage.ColorIndexAt(pt.X, pt.Y) == 1 {
					count++
				}
				return false
			})

			// Zero out the image. This will be replaced with memclr by the
			// compiler which is way faster.
			for i := range poissonImage.Pix {
				poissonImage.Pix[i] = 0
			}

			log.Printf("for radius %f, got %d points", r, count)
			return count <= numLED
		},
	)

	poissonPoints := poissondisc.Sample(
		float64(poissonImage.Bounds().Min.X),
		float64(poissonImage.Bounds().Min.Y),
		float64(poissonImage.Bounds().Max.X),
		float64(poissonImage.Bounds().Max.Y),
		float64(poissonRadius), 10000, rand)

	poissonMap := make(map[image.Point]struct{}, len(poissonPoints))
	for _, ppt := range poissonPoints {
		pt := image.Pt(int(math.Round(ppt.X)), int(math.Round(ppt.Y)))
		poissonMap[pt] = struct{}{}
	}

	ledPoints := make([]image.Point, 0, numLED)
	ledImage := &image.Paletted{
		Pix:    make([]uint8, boundaryBox.Dx()*boundaryBox.Dy()),
		Stride: boundaryBox.Dx(),
		Rect: image.Rectangle{
			Min: image.ZP,
			Max: boundaryBox.Size(),
		},
		Palette: []color.Color{
			color.Black,
			color.White,
		},
	}

	boundaryImage.EachPt(func(pt image.Point) bool {
		if _, ok := poissonMap[pt]; ok {
			// Translate the point to the boundary box.
			pt = pt.Sub(boundaryBox.Min)

			ledPoints = append(ledPoints, pt)
			xdraw.EachCirclePx(pt, ledSize, func(pt image.Point) bool {
				ledImage.SetColorIndex(pt.X, pt.Y, 1)
				return false
			})
		}

		return false
	})

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	if csvName != "" {
		csvPath := filepath.Join(outDir, csvName)
		log.Println("writing CSV file to", csvPath)

		if err := csvutil.MarshalFile(csvPath, ledPoints); err != nil {
			return errors.Wrap(err, "failed to marshal CSV file")
		}
	}

	if pngName != "" {
		pngPath := filepath.Join(outDir, pngName)
		log.Println("writing PNG image to", pngPath)

		pngFile, err := os.Create(pngPath)
		if err != nil {
			return errors.Wrap(err, "failed to create PNG file")
		}
		defer pngFile.Close()

		if err := png.Encode(pngFile, ledImage); err != nil {
			return errors.Wrap(err, "failed to encode PNG file")
		}

		if err := pngFile.Close(); err != nil {
			return errors.Wrap(err, "failed to close PNG file")
		}
	}

	return nil
}

func decodeImageFile(file string) (image.Image, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode image")
	}

	return img, nil
}

func searchFloat(min, max, acc float64, fn func(float64) bool) float64 {
	for min < max {
		mid := (min + max) / 2
		if !fn(mid) {
			min = mid + acc
		} else {
			max = mid
		}
	}
	return min
}
