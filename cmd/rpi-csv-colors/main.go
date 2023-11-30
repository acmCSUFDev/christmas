package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"time"

	"libdb.so/acm-christmas/internal/csvutil"
	"libdb.so/acm-christmas/internal/xcolor"
	"libdb.so/ledctl"
)

func main() {
	log.SetFlags(0)

	flag.Usage = func() {
		log.Println("rpi-csv-colors renders a CSV file of colors to LED lights.")
		log.Println("Usage: rpi-csv-colors <csv-colors-file>")
	}
	flag.Parse()

	csvColorsFile := flag.Arg(0)
	if csvColorsFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	colors, err := csvutil.UnmarshalFile[xcolor.RGB](csvColorsFile)
	if err != nil {
		log.Fatalln("failed to unmarshal CSV colors:", err)
	}

	log.Println("got", len(colors), "LED lights")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	strip, err := ledctl.NewWS281x(ledctl.WS281xConfig{
		NumPixels:    len(colors),
		ColorOrder:   ledctl.BGROrder,
		ColorModel:   ledctl.RGBModel,
		PWMFrequency: 800000,
		DMAChannel:   10,
		GPIOPins:     []int{12},
	})
	if err != nil {
		log.Fatalln("failed to create pixarray:", err)
	}

	for i, color := range colors {
		strip.SetRGBAt(i, ledctl.RGB(color))
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := strip.Flush(); err != nil {
				log.Fatalln("failed to write pixels:", err)
			}
		}
	}
}
