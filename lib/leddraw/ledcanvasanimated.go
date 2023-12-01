package leddraw

import (
	"context"
	"fmt"
	"image"
	"sync"

	"dev.acmcsuf.com/christmas/internal/animation"
)

// LEDCanvas wraps an LEDCanvas and provides animation capabilities.
// Frames are sent to the C channel.
type LEDCanvasAnimated struct {
	C <-chan animation.Frame[LEDStrip]

	player *animation.Player[LEDStrip]
	canvas *LEDCanvas
	adding sync.Mutex // lock
	opts   LEDCanvasOpts
}

// NewLEDCanvasAnimated creates a new LEDCanvasAnimated.
func NewLEDCanvasAnimated(ledPositions []image.Point, opts LEDCanvasOpts) (*LEDCanvasAnimated, error) {
	canvas, err := NewLEDCanvas(ledPositions, opts)
	if err != nil {
		return nil, err
	}

	c := &LEDCanvasAnimated{
		player: animation.NewPlayer[LEDStrip](),
		canvas: canvas,
		opts:   opts,
	}
	c.C = c.player.C
	return c, nil
}

// Run starts the animated canvas player.
func (c *LEDCanvasAnimated) Run(ctx context.Context) error {
	return c.player.Run(ctx)
}

// AddFrames adds frames to the animated canvas.
func (c *LEDCanvasAnimated) AddFrames(ctx context.Context, images []animation.Frame[*image.RGBA]) error {
	if !c.adding.TryLock() {
		return fmt.Errorf("cannot add frames: already adding frames")
	}
	defer c.adding.Unlock()

	for i, frame := range images {
		rendered, err := renderCanvas(c.canvas, frame)
		if err != nil {
			return fmt.Errorf("cannot render frame %d: %w", i, err)
		}
		if err := c.player.AddFrame(ctx, rendered); err != nil {
			return fmt.Errorf("cannot add frame %d: %w", i, err)
		}
	}

	return nil
}

func renderCanvas(canvas *LEDCanvas, frame animation.Frame[*image.RGBA]) (animation.Frame[LEDStrip], error) {
	if err := canvas.Render(frame.Image); err != nil {
		return animation.Frame[LEDStrip]{}, err
	}

	return animation.Frame[LEDStrip]{
		Image:          append(LEDStrip(nil), canvas.LEDs()...),
		JumpBackAmount: frame.JumpBackAmount,
		DurationMs:     frame.DurationMs,
	}, nil
}
