package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"github.com/spf13/pflag"
)

//go:embed README
var readme string

var (
	startTime   = MustParseTime("00:00:00")
	startFrame  = -1
	wormSpeed   = 200 * time.Millisecond
	numLEDs     = 100
	addHalfStep = true
	filters     = []string{}
)

func init() {
	log.SetFlags(0)
}

func main() {
	pflag.Usage = func() {
		log.Println(readme)
		log.Printf("Usage:")
		log.Printf("  %s [options] <input-file> [<output-dir>]", os.Args[0])
		log.Printf("")
		log.Printf("Options:")
		pflag.PrintDefaults()
	}

	pflag.VarP(&startTime, "start-time", "s", "Start time in H:M:S, M:S or S format")
	pflag.IntVarP(&startFrame, "start-frame", "S", startFrame, "Start frame, used if not -1")
	pflag.DurationVarP(&wormSpeed, "worm-speed", "w", wormSpeed, "Worm speed")
	pflag.IntVarP(&numLEDs, "num-leds", "n", numLEDs, "Number of LEDs")
	pflag.BoolVarP(&addHalfStep, "add-half-step", "a", addHalfStep, "Add half a step to the start time")
	pflag.StringSliceVarP(&filters, "filter", "f", filters, "Additional ffmpeg filters")
	pflag.Parse()

	inputFile := pflag.Arg(0)
	if inputFile == "" {
		pflag.Usage()
		os.Exit(1)
	}

	outputDir := pflag.Arg(1)
	if outputDir == "" {
		d, err := os.MkdirTemp(os.TempDir(), "extract-frames-")
		if err != nil {
			log.Fatalln("failed to create temp dir:", err)
		}
		outputDir = d
		log.Println("Output directory not specified, using", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalln("failed to create output directory:", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := extractFrames(ctx, inputFile, outputDir); err != nil {
		log.Fatalln("failed to extract frames:", err)
	}
}

func extractFrames(ctx context.Context, inputFile, outputDir string) error {
	frameDuration, err := probeVideoFrameDuration(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("failed to probe video frame rate: %w", err)
	}

	// Calculate the number of frames (steps) between each LED update.
	// TODO: don't round this, just count
	frameStep := float64(wormSpeed) / float64(frameDuration)

	if addHalfStep {
		startTime += Time(float64(frameDuration) * frameStep / 2)
	}

	trimDuration := Time(float64(frameDuration) * frameStep * float64(numLEDs))

	filter := fmt.Sprintf(`select=not(mod(n\,%f))`, math.Round(frameStep))
	if len(filters) > 0 {
		filter += ","
		filter += strings.Join(filters, ",")
	}

	if startFrame > 0 {
		// Doing start frame in FFmpeg is kind of a pain, so we just
		// calculate the start time and trim duration.
		startTime = Time(float64(frameDuration) * float64(startFrame))
	}

	ffFlags := []string{
		"-loglevel", "error",
		"-hide_banner",
		"-ss", startTime.String(),
		"-t", trimDuration.String(),
		"-i", inputFile,
		"-vf", filter,
		"-vsync", "vfr",
		"-q:v", "1",
		filepath.Join(outputDir, "frame-%05d.png"),
	}

	if _, err := run(ctx, "ffmpeg", ffFlags...); err != nil {
		return fmt.Errorf("failed to extract frames: %w", err)
	}

	return nil
}

func probeVideoFrameDuration(ctx context.Context, videoFile string) (time.Duration, error) {
	out, err := run(ctx,
		"ffprobe",
		"-v", "error",
		"-select_streams", "v",
		"-of", "default=noprint_wrappers=1:nokey=1",
		"-show_entries", "stream=avg_frame_rate", videoFile)
	if err != nil {
		return 0, err
	}

	var num, denom int
	if _, err := fmt.Sscanf(out, "%d/%d", &num, &denom); err != nil {
		return 0, fmt.Errorf("failed to parse frame rate: %w", err)
	}

	fps := float64(num) / float64(denom)
	return time.Duration(float64(time.Second) / fps), nil
}

func run(ctx context.Context, arg0 string, argv ...string) (string, error) {
	log.Println(">", arg0, strings.Join(argv, " "))
	var out strings.Builder
	cmd := exec.CommandContext(ctx, arg0, argv...)
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s: %s", arg0, exitErr.Stderr)
		}
		return "", fmt.Errorf("%s: %w", arg0, err)
	}
	return strings.TrimSpace(out.String()), nil
}
