package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"

	_ "embed"
	_ "image/jpeg"
	_ "image/png"

	"dev.acmcsuf.com/christmas/lib/csvutil"
	"dev.acmcsuf.com/christmas/lib/vision"
	"dev.acmcsuf.com/christmas/lib/xdraw"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	_ "golang.org/x/image/bmp"
)

//go:embed README
var readme string

var (
	maskFile  = ""
	maskColor = colorFlag{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	spotColor = colorFlag{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	outDir    = ""
	csvName   = "led-points.csv"
	outputPNG = false
	maxJobs   = runtime.NumCPU()
)

func init() {
	log.SetFlags(0)
}

func main() {
	pflag.Usage = func() {
		log.Println(readme)
		log.Printf("Usage:")
		log.Printf("  %s [options] <input-file-or-directory>", os.Args[0])
		log.Printf("")
		log.Printf("Options:")
		pflag.PrintDefaults()
	}

	pflag.StringVarP(&maskFile, "mask", "M", maskFile, "Image mask file, acts as a boundary")
	pflag.VarP(&maskColor, "mask-color", "C", "Color of the mask, in hex format")
	pflag.VarP(&spotColor, "spot-color", "c", "Color of the spots, in hex format")
	pflag.IntVarP(&maxJobs, "max-jobs", "j", maxJobs, "Maximum number of concurrent jobs")
	pflag.StringVar(&csvName, "csv-name", csvName, "Points CSV output file name")
	pflag.StringVarP(&outDir, "out-dir", "o", outDir, "Output directory, empty to use temp dir")
	pflag.BoolVar(&outputPNG, "output-png", outputPNG, "Output PNG files")
	pflag.Parse()

	if maskFile != "" {
		log.Fatalln("mask is not implemented yet")
	}

	if pflag.NArg() == 0 {
		pflag.Usage()
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	files := make([]string, 0, pflag.NArg())
	for _, file := range pflag.Args() {
		stat, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("failed to stat %q: %w", file, err)
		}

		if stat.IsDir() {
			d, err := os.ReadDir(file)
			if err != nil {
				return fmt.Errorf("failed to read dir %q: %w", file, err)
			}
			for _, f := range d {
				if f.IsDir() {
					continue
				}
				files = append(files, filepath.Join(file, f.Name()))
			}
		} else {
			files = append(files, file)
		}
	}

	// var maskImage *vision.BoundaryImage
	// if maskFile != "" {
	// 	img, err := decodeImageFile(maskFile)
	// 	if err != nil {
	// 		return fmt.Errorf("failed to decode mask image: %w", err)
	// 	}
	// 	maskImage = vision.NewBoundaryImage(img, maskColor.AsColor())
	// }

	errg, ctx := errgroup.WithContext(ctx)
	defer errg.Wait()

	fileCh := make(chan string)
	resultCh := make(chan processingResult)

	for i := 0; i < maxJobs; i++ {
		i := i
		errg.Go(func() error {
			processor := newProcessor()
			istr := padDigits(i, maxJobs)

			for file := range fileCh {
				log.Printf("%s: processing %s", istr, file)

				result, err := processor.process(ctx, file)
				if err != nil {
					log.Printf("%s: %s: !!! WARNING !!!: NO SPOT FOUND: %s", istr, file, err)
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case resultCh <- result:
				}
			}

			return nil
		})
	}

	result := make([]processingResult, 0, len(files))
	errg.Go(func() error {
		for range files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case r := <-resultCh:
				result = append(result, r)
			}
		}
		return nil
	})

	errg.Go(func() error {
		defer close(fileCh)
		for _, file := range files {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case fileCh <- file:
			}
		}
		return nil
	})

	if err := errg.Wait(); err != nil {
		return err
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].File < result[j].File
	})

	for i := range result {
		if result[i].Spot.Area == 0 && i > 0 {
			// If there is no spot, use the previous spot.
			result[i] = result[i-1]
		}
	}

	boundingBox := findBoundingBox(result)
	// Translate all points to the top left corner of the bounding box.
	for i := range result {
		result[i].Spot.Center = result[i].Spot.Center.Sub(boundingBox.Min)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	if csvName != "" {
		if err := createCSVOutput(result); err != nil {
			return errors.Wrap(err, "failed to create CSV output")
		}
	}

	if outputPNG {
		if err := createPNGOutput(result, boundingBox); err != nil {
			return errors.Wrap(err, "failed to create PNG output")
		}
	}

	return nil
}

func findBoundingBox(results []processingResult) image.Rectangle {
	pts := make([]image.Point, 0, len(results))
	for _, result := range results {
		pts = append(pts, result.Spot.Center)
	}
	return xdraw.BoundingBox(pts)
}

func createCSVOutput(results []processingResult) error {
	csvPath := filepath.Join(outDir, csvName)
	log.Println("writing CSV file to", csvPath)

	type record struct {
		X int
		Y int
		// Area int
	}

	records := make([]record, 0, len(results))
	for _, r := range results {
		records = append(records, record{
			X: r.Spot.Center.X,
			Y: r.Spot.Center.Y,
			// Area: r.Spot.Area,
		})
	}

	if err := csvutil.MarshalFile(csvPath, records); err != nil {
		return errors.Wrap(err, "failed to marshal CSV file")
	}
	return nil
}

func createPNGOutput(results []processingResult, boundingBox image.Rectangle) error {
	log.Println("writing PNG images to", outDir)

	for i, r := range results {
		pngPath := filepath.Join(outDir, padDigits(i, len(results))+".png")

		pngFile, err := os.Create(pngPath)
		if err != nil {
			return errors.Wrap(err, "failed to create PNG file")
		}
		defer pngFile.Close()

		img := xdraw.SubImage(r.Spot.Filled, boundingBox)
		if err := png.Encode(pngFile, img); err != nil {
			return errors.Wrap(err, "failed to encode PNG file")
		}

		if err := pngFile.Close(); err != nil {
			return errors.Wrap(err, "failed to close PNG file")
		}
	}

	return nil
}

func padDigits(n, max int) string {
	numDigits := int(math.Log10(float64(max))) + 1
	numf := fmt.Sprintf("%%0%dd", numDigits)
	return fmt.Sprintf(numf, n)
}

type processor struct {
	spots *vision.SpotFinder
}

type processingResult struct {
	File string
	Spot vision.BigSpot
}

var blankImage = image.NewRGBA(image.Rect(0, 0, 1, 1))

func newProcessor() *processor {
	return &processor{
		spots: vision.NewSpotFinder(blankImage),
	}
}

func (p *processor) process(ctx context.Context, inputImage string) (processingResult, error) {
	result := processingResult{File: inputImage}

	img, err := decodeImageFile(inputImage)
	if err != nil {
		return result, fmt.Errorf("failed to decode image: %w", err)
	}

	p.spots.Reset(img)

	biggest, err := p.spots.FindBiggestSpot(spotColor.AsColor())
	if err != nil {
		return result, fmt.Errorf("failed to find biggest spot: %w", err)
	}

	result.Spot = biggest
	return result, nil
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
