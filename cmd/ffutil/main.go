package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"os/signal"
	"strconv"

	"dev.acmcsuf.com/christmas/lib/ffutil"
)

func threshold(ctx context.Context, args [2]string) error {
	var size image.Point
	_, err := fmt.Sscanf(args[0], "%dx%d", &size.X, &size.Y)
	if err != nil {
		return fmt.Errorf("failed to parse size: %w", err)
	}

	threshold, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		return fmt.Errorf("failed to parse threshold: %w", err)
	}

	if threshold < 0 || threshold > 1 {
		return fmt.Errorf("threshold must be in [0, 1]")
	}

	fmt.Println(ffutil.MakeThreshold(size, threshold))
	return nil
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var err error
	switch flag.Arg(0) {
	case "threshold":
		err = threshold(ctx, [2]string{flag.Arg(1), flag.Arg(2)})
	case "":
		log.Println("Commands:")
		log.Println("  threshold <size> <threshold>")
	default:
		log.Fatalln("unknown command:", flag.Arg(0))
	}

	if err != nil {
		log.Fatalln(err)
	}
}
