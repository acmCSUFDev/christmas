package animation

import (
	"context"
	"errors"
	"expvar"
	"log"
	"time"

	"gopkg.in/typ.v4/lists"
)

// Milliseconds is a duration in milliseconds.
type Milliseconds uint32

// DurationToMs converts a time.Duration to Milliseconds. The duration is
// rounded down to the nearest millisecond.
func DurationToMs(d time.Duration) Milliseconds {
	return Milliseconds(d / time.Millisecond)
}

// Frame is a single frame of an animation.
type Frame[Image any] struct {
	Image          Image
	JumpBackAmount int32
	DurationMs     Milliseconds
}

// Duration returns the duration of the frame as a time.Duration.
func (f Frame[Image]) Duration() time.Duration {
	return time.Duration(f.DurationMs) * time.Millisecond
}

// ErrFramebufferOverflow is returned when the framebuffer is full.
var ErrFramebufferOverflow = errors.New("framebuffer overflow")

const (
	metricDroppedFrames = "dropped_frames"
	metricTotalFrames   = "total_frames"
	metricFrameJitter   = "frame_jitter"
)

var metrics = expvar.NewMap("animation")

// Player is an animation player. It is safe to use from multiple goroutines.
type Player[Image any] struct {
	C <-chan Frame[Image]

	ch    chan Frame[Image]
	addCh chan Frame[Image] // nil clears

	insert   *lists.Ring[Frame[Image]] // points to empty slot next to last frame
	playback *lists.Ring[Frame[Image]] // points to current frame
}

// NewPlayer creates a new animation player that can hold up to maxFrames
// frames.
func NewPlayer[Image any]() *Player[Image] {
	return NewPlayerWithSize[Image](100)
}

// NewPlayerWithSize creates a new animation player that can hold up to
// maxFrames frames.
func NewPlayerWithSize[Image any](maxFrames int) *Player[Image] {
	if maxFrames < 2 {
		panic("maxFrames must be at least 2")
	}

	ch := make(chan Frame[Image])
	frames := lists.NewRing[Frame[Image]](maxFrames)

	return &Player[Image]{
		C:        ch,
		ch:       ch,
		addCh:    make(chan Frame[Image]),
		insert:   frames,
		playback: frames.Prev(),
	}
}

// AddFrames adds a frame to the animation. If the player is full, the function
// blocks until there is room for the frame.
func (p *Player[Image]) AddFrame(ctx context.Context, frame Frame[Image]) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.addCh <- frame:
		return nil
	}
}

// AddFrames adds multiple frames to the animation. If the player is full, the
// function blocks until there is room for the frames.
func (p *Player[Image]) AddFrames(ctx context.Context, frames []Frame[Image]) error {
	for _, frame := range frames {
		if err := p.AddFrame(ctx, frame); err != nil {
			return err
		}
	}
	return nil
}

// Run starts playing the animation. Run returns when the animation is
// finished or when the context is canceled.
func (p *Player[Image]) Run(ctx context.Context) error {
	var frameCh chan Frame[Image]
	addCh := p.addCh

	var currentFrame Frame[Image]
	var nextFrame *Frame[Image]

	nextFrameTimer := time.NewTimer(0)
	defer nextFrameTimer.Stop()

	if !nextFrameTimer.Stop() {
		<-nextFrameTimer.C
	}

	scheduleNextFrame := func() {
		if nextFrame != nil {
			panic("scheduleNextFrame called but nextFrame is still not used")
		}

		f, ok := p.nextFrame()
		log.Printf("schedule next frame: %v", f)
		if ok {
			nextFrameTimer.Reset(f.Duration())
			nextFrame = f
		} else {
			nextFrameTimer.Stop()
			nextFrame = nil
		}

		// We can take more frames now, so unblock the addCh.
		addCh = p.addCh
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case frame := <-addCh:
			p.addFrame(frame)
			if p.isFull() {
				addCh = nil
			}

			if nextFrame == nil {
				// No frame is currently being played, so start playing the
				// first frame.
				scheduleNextFrame()
			}

		case <-nextFrameTimer.C:
			if nextFrame == nil {
				panic("unreachable: nextFrameTimer fired but nextFrame is nil")
			}

			if frameCh != nil {
				// Timer for next frame fired, but the previous frame hasn't
				// been sent yet. This means that the receiver is too slow.
				metrics.Add(metricDroppedFrames, 1)
			}

			currentFrame, nextFrame = *nextFrame, nil
			frameCh = p.ch

			// Advancing the frame here instead of waiting for the receiver
			// to pick up the frame. This ensures that the animation is
			// played at the correct speed even if the receiver is slow.
			scheduleNextFrame()

		case frameCh <- currentFrame:
			frameCh = nil
			metrics.Add(metricTotalFrames, 1)
		}
	}
}

// addFrame adds a frame to the player. If the player is already full, false is
// returned and the player halts.
func (p *Player[Image]) addFrame(f Frame[Image]) {
	p.insert.Value = f
	p.insert = p.insert.Next()
}

func (p *Player[Image]) clearFrames() {
	p.playback = p.insert.Prev()
}

// nextFrame returns the next frame in the animation. False is returned if the
// animation is finished.
func (p *Player[Image]) nextFrame() (*Frame[Image], bool) {
	current := p.playback

	if current.Value.JumpBackAmount > 0 {
		jumpBackAmount := current.Value.JumpBackAmount
		for i := int32(0); i < jumpBackAmount; i++ {
			current = current.Prev()
			if current == p.insert {
				return nil, false
			}
		}
	} else {
		current = current.Next()
		if current == p.insert {
			return nil, false
		}
	}

	p.playback = current
	return &current.Value, true
}

// isFull returns true if the player cannot take in any more frames.
func (p *Player[Image]) isFull() bool {
	return p.insert == p.playback
}
