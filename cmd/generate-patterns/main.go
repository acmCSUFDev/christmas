package main

import (
	"encoding/json"
	"fmt"
	"image"
	"log"
	"os"
	"sort"

	"github.com/spf13/pflag"
	"dev.acmcsuf.com/christmas/internal/csvutil"
)

var (
	ledPoints = "led-points.csv"
	format    = "json"
)

func main() {
	log.SetFlags(0)

	pflag.Usage = func() {
		log.Printf("generate-patterns generates patterns for the LED strips.")
		log.Printf("")
		log.Printf("Usage:")
		log.Printf("  %s [options] <pattern>", os.Args[0])
		log.Printf("")
		log.Printf("Patterns:")
		for _, p := range listPatterns() {
			log.Printf("  %s", p)
		}
		log.Printf("")
		log.Printf("Options:")
		pflag.PrintDefaults()
	}

	pflag.StringVarP(&ledPoints, "led-points", "i", ledPoints, "path to the CSV file containing the LED points")
	pflag.StringVarP(&format, "format", "f", format, "output format (json, go)")
	pflag.Parse()

	if err := do(); err != nil {
		log.Fatalln(err)
	}
}

var patterns = map[string]func() error{
	"scan-up": scanUp,
}

func listPatterns() []string {
	var out []string
	for k := range patterns {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func do() error {
	if pflag.NArg() != 1 {
		pflag.Usage()
		os.Exit(1)
	}

	fn, ok := patterns[pflag.Arg(0)]
	if !ok {
		return fmt.Errorf("unknown pattern: %q", pflag.Arg(0))
	}
	if err := fn(); err != nil {
		return fmt.Errorf("%s: %w", pflag.Arg(0), err)
	}
	return nil
}

func scanUp() error {
	pts, err := csvutil.UnmarshalFile[image.Point](ledPoints)
	if err != nil {
		return fmt.Errorf("failed to read LED points: %w", err)
	}

	type ledPt struct {
		image.Point
		Index int
	}

	ledPts := make([]ledPt, len(pts))
	for i, pt := range pts {
		ledPts[i] = ledPt{pt, i}
	}

	sort.SliceStable(ledPts, func(i, j int) bool {
		return ledPts[i].Y > ledPts[j].Y
	})

	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ledPts)
	case "go":
		fmt.Println("var ledOrder = []int{")
		for _, pt := range ledPts {
			fmt.Printf("\t%d,\n", pt.Index)
		}
		fmt.Println("}")
		return nil
	default:
		return fmt.Errorf("unknown format: %q", format)
	}
}
