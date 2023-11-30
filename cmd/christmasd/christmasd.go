package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/spf13/pflag"
	"libdb.so/acm-christmas/internal/csvutil"
	"libdb.so/ledctl"
)

var (
	ledPointsCSV = "led-points.csv"
	canvasPPI    = 72.0
	verbose      = false
)

func init() {
	pflag.StringVar(&ledPointsCSV, "led-points", ledPointsCSV, "CSV file of LED points")
	pflag.Float64Var(&canvasPPI, "canvas-ppi", canvasPPI, "canvas PPI")
	pflag.BoolVarP(&verbose, "verbose", "v", verbose, "verbose logging")
}

const frameRate = 20

var ws281xConfig = ledctl.WS281xConfig{
	ColorOrder:   ledctl.BGROrder,
	ColorModel:   ledctl.RGBModel,
	PWMFrequency: 800000,
	DMAChannel:   10,
	GPIOPins:     []int{12},
}

func main() {
	log.SetFlags(0)
	pflag.Parse()

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	logHandler := tint.NewHandler(os.Stderr, &tint.Options{
		Level:      level,
		TimeFormat: "15:04:05 PM", // extended time.Kitchen
		NoColor:    !isatty.IsTerminal(os.Stderr.Fd()),
	})

	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, logger); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	ledCoords, err := csvutil.UnmarshalFile[image.Point](ledPointsCSV)
	if err != nil {
		return fmt.Errorf("failed to unmarshal CSV file %q: %v", ledPointsCSV, err)
	}

	ws281xCfg := ws281xConfig
	ws281xCfg.NumPixels = len(ledCoords)

	ws281x, err := ledctl.NewWS281x(ws281xCfg)
	if err != nil {
		return fmt.Errorf("failed to create a WS281x controller: %v", err)
	}

	controller, err := newLEDController(ledControlConfig{
		Controller: ws281x,
		LEDCoords:  ledCoords,
		FrameRate:  frameRate,
		CanvasPPI:  canvasPPI,
		Logger:     logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create a LED controller: %v", err)
	}

	return nil
}
