package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
	"dev.acmcsuf.com/christmas/internal/quoted"
	"dev.acmcsuf.com/christmas/lib/livecapture"
)

var (
	camerarc   = "camerarc"
	imagePath  = "/run/user/1000/camera.bmp"
	filterArgs string
	image2Args string
)

func init() {
	pflag.StringVarP(&camerarc, "camerarc", "c", camerarc, "path to the camera rc file")
	pflag.StringVarP(&imagePath, "image-path", "p", imagePath, "path to the image file")
	pflag.StringVar(&filterArgs, "filter-args", filterArgs, "args to pass to the filter")
	pflag.StringVar(&image2Args, "image2-args", image2Args, "args to pass to image2")
}

func newCapture() *livecapture.Capture {
	rc, err := godotenv.Read(camerarc)
	if err != nil {
		log.Fatalln("failed to read camerarc:", err)
	}

	for k := range rc {
		if v, ok := os.LookupEnv("CAMERA_" + k); ok {
			k = strings.TrimPrefix(k, "CAMERA_")
			rc[k] = v
		}
	}

	for k, v := range rc {
		log.Printf("Using CAMERA_%s=%q", k, v)
	}

	var size image.Point

	_, err = fmt.Sscanf(rc["SIZE"], "%dx%d", &size.X, &size.Y)
	must("failed to parse CAMERA_SIZE:", err)

	frameRate, err := strconv.Atoi(rc["FRAMERATE"])
	must("failed to parse CAMERA_FRAMERATE:", err)

	filterArgs, err := quoted.Split(filterArgs)
	must("failed to parse filter args:", err)

	image2Args, err := quoted.Split(image2Args)
	must("failed to parse image2 args:", err)

	opts := livecapture.CaptureOpts{
		Camera: livecapture.Camera{
			Path:      rc["PATH"],
			Size:      size,
			Format:    livecapture.CameraFormat(rc["FORMAT"]),
			FrameRate: frameRate,
		},
		ImagePath:  imagePath,
		FilterArgs: filterArgs,
		Image2Args: image2Args,
	}

	c, err := livecapture.NewCapture(opts)
	must("failed to create capture:", err)

	return c
}

func start(ctx context.Context) error {
	capture := newCapture()

	log.Println("Using", imagePath, "as image destination.")
	log.Println("To read a frame, open this file and use that file descriptor.")

	return capture.Start(ctx)
}

func wait(ctx context.Context) error {
	capture := newCapture()
	return capture.WaitForFile(ctx)
}

func snapshot(ctx context.Context, dst string) error {
	if dst == "" {
		return errors.New("usage: snapshot <dst>")
	}

	capture := newCapture()
	return capture.SnapshotToFile(ctx, dst)
}

func view(ctx context.Context) error {
	capture := newCapture()
	return capture.View(ctx)
}

const usage = `live-capture <start|wait|snapshot|view> [args...]`

func main() {
	log.SetFlags(0)

	pflag.Usage = func() {
		log.Print("Usage:")
		log.Print("  ", usage)
		log.Print()
		log.Print("Flags:")
		pflag.PrintDefaults()
	}

	pflag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var err error
	switch pflag.Arg(0) {
	case "start":
		err = start(ctx)
	case "wait":
		err = wait(ctx)
	case "snapshot":
		err = snapshot(ctx, pflag.Arg(1))
	case "view":
		err = view(ctx)
	default:
		log.Println("unknown command:", pflag.Arg(0))
		log.Fatalln("usage:", usage)
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func must(args ...any) {
	if args[len(args)-1] != nil {
		log.Fatalln(args...)
	}
}
