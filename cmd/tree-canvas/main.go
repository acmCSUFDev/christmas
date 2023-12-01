package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/jpeg"
	_ "image/png"

	"dev.acmcsuf.com/christmas/lib/csvutil"
	"dev.acmcsuf.com/christmas/lib/leddraw"
	"dev.acmcsuf.com/christmas/lib/xdraw"
	"github.com/disintegration/imaging"
	"github.com/spf13/pflag"

	_ "golang.org/x/image/bmp"
)

// TODO: make this take in an image and draws this image onto a bunch of LED
// points.

var (
	ledPointsFile = "led-points.csv"
	outputDir     = "."
	pngImageFile  = ""
	csvColorFile  = ""
	goCodeFile    = ""
	maxPtDistance = 0.0 // auto
	ppi           = 72.0
	fit           = false
)

func init() {
	pflag.StringVarP(&ledPointsFile, "led-points", "i", ledPointsFile, "path to the CSV file containing the LED points")
	pflag.StringVarP(&outputDir, "output-dir", "o", outputDir, "path to the output directory")
	pflag.StringVar(&pngImageFile, "png-image", pngImageFile, "path to the output PNG image file")
	pflag.StringVar(&csvColorFile, "csv-color", csvColorFile, "path to the output CSV color file")
	pflag.StringVar(&goCodeFile, "go-code", goCodeFile, "path to the output Go code file")
	pflag.Float64Var(&maxPtDistance, "max-distance", maxPtDistance, "maximum distance between a point and an LED")
	pflag.Float64Var(&ppi, "ppi", ppi, "pixels per inch")
	pflag.BoolVar(&fit, "fit", fit, "fill or fit the source image (default: fill)")
}

func main() {
	pflag.Parse()

	ledPoints, err := readCSVPoints(ledPointsFile)
	if err != nil {
		log.Fatalln("failed to read LED points:", err)
	}

	var ledCanvasOpts leddraw.LEDCanvasOpts
	ledCanvasOpts.PPI = ppi
	if maxPtDistance > 0 {
		ledCanvasOpts.Intensity = leddraw.NewCubicIntensity(maxPtDistance)
	}

	ledCanvas, err := leddraw.NewLEDCanvas(ledPoints, ledCanvasOpts)
	if err != nil {
		log.Fatalln("failed to create LED canvas:", err)
	}

	// TODO: scale while preserving aspect ratio, and center the image.
	canvasBounds := ledCanvas.CanvasBounds()

	img, err := decodeImageFile(pflag.Arg(0))
	if err != nil {
		log.Fatalln("failed to decode source image:", err)
	}

	var imagedCanvas *image.NRGBA
	if fit {
		imagedCanvas = imaging.Fit(
			img, canvasBounds.Dx(), canvasBounds.Dy(), imaging.NearestNeighbor)
	} else {
		imagedCanvas = imaging.Fill(
			img, canvasBounds.Dx(), canvasBounds.Dy(), imaging.Center, imaging.NearestNeighbor)
	}
	imagedCanvas = imaging.PasteCenter(image.NewNRGBA(canvasBounds), imagedCanvas)

	start := time.Now()
	if err := ledCanvas.Render((*image.RGBA)(imagedCanvas)); err != nil {
		log.Fatalln("failed to render image:", err)
	}
	log.Println("rendered in", time.Since(start))

	if pngImageFile != "" {
		if err := writePNGImage(ledCanvas, ledPoints); err != nil {
			log.Fatalln("failed to write PNG image:", err)
		}
	}

	if csvColorFile != "" {
		if err := writeCSVColors(ledCanvas, ledPoints); err != nil {
			log.Fatalln("failed to write CSV colors:", err)
		}
	}

	if goCodeFile != "" {
		if err := writeGoCode(ledCanvas, ledPoints); err != nil {
			log.Fatalln("failed to write Go code:", err)
		}
	}

	if pngImageFile == "" && csvColorFile == "" && goCodeFile == "" {
		log.Println("Nothing to do.")
		log.Println()
		log.Println("Debug Information:")
		log.Println("  LED bounds:  ", ledCanvas.LEDBounds())
		log.Println("  Canvas size: ", canvasBounds.Dx(), "x", canvasBounds.Dy())
		log.Println()
	}
}

func writePNGImage(ledCanvas *leddraw.LEDCanvas, ledPoints []image.Point) error {
	outImage := image.NewRGBA(ledCanvas.LEDBounds())
	draw.Draw(outImage, outImage.Bounds(), image.NewUniform(color.Black), image.ZP, draw.Src)

	// TODO: write these points to a CSV file.
	// TODO: write these points to a []color.RGBA file.
	for i, led := range ledCanvas.LEDs() {
		xdraw.DrawCircle(outImage, ledPoints[i], 3, led)
	}

	f, err := createFile(pngImageFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, outImage); err != nil {
		return fmt.Errorf("failed to encode PNG: %v", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %v", err)
	}

	return nil
}

func writeCSVColors(ledCanvas *leddraw.LEDCanvas, ledPoints []image.Point) error {
	type color struct {
		R, G, B uint8
	}

	colors := make([]color, len(ledCanvas.LEDs()))
	for i, led := range ledCanvas.LEDs() {
		colors[i] = color{led.R, led.G, led.B}
	}

	f, err := createFile(csvColorFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	if err := csvutil.Marshal(csv.NewWriter(f), colors); err != nil {
		return fmt.Errorf("failed to marshal CSV: %v", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %v", err)
	}

	return nil
}

func writeGoCode(ledCanvas *leddraw.LEDCanvas, ledPoints []image.Point) error {
	var buf bytes.Buffer
	buf.WriteString("var ledColors = []color.RGBA{")
	for _, led := range ledCanvas.LEDs() {
		buf.WriteString(fmt.Sprintf("{0x%02X, 0x%02X, 0x%02X, 0xFF}, ", led.R, led.G, led.B))
	}
	buf.WriteString("}\n")

	f, err := createFile(goCodeFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	if _, err := buf.WriteTo(f); err != nil {
		return fmt.Errorf("failed to write to output file: %v", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close output file: %v", err)
	}

	return nil
}

func createFile(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdout, nil
	}

	if err := os.MkdirAll(filepath.Dir(outputDir), 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	if strings.HasPrefix(name, "/") {
		return os.Create(name)
	}

	return os.Create(filepath.Join(outputDir, name))
}

func readCSVPoints(csvPath string) ([]image.Point, error) {
	pts, err := csvutil.UnmarshalFile[image.Point](csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal CSV file %q: %v", csvPath, err)
	}
	return pts, nil
}

func decodeImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}
