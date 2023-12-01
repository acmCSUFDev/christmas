package animation

import (
	"context"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	"dev.acmcsuf.com/christmas/internal/intmath"
)

type testFrame struct {
	data string
}

// Ensure that each frame gets shown for exactly that long with a margin of
// this amount. Allow 5ms to allow tests to run on slow machines.
const frameTimeMargin = 5 * time.Millisecond

func TestPlayer(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		p, _ := startPlayer(t, 10)
		mustAddFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
		expectFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
	})

	t.Run("loop", func(t *testing.T) {
		p, _ := startPlayer(t, 10)
		mustAddFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 3, 250},
		})
		expectFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 3, 250},
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 3, 250},
		})
	})

	t.Run("race", func(t *testing.T) {
		p, _ := startPlayer(t, 10)
		go mustAddFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
		expectFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
	})

	t.Run("interleave", func(t *testing.T) {
		p, _ := startPlayer(t, 10)
		mustAddFrames(t, p, []Frame[testFrame]{{testFrame{"frame 1"}, 0, 100}})
		expectFrames(t, p, []Frame[testFrame]{{testFrame{"frame 1"}, 0, 100}})
		mustAddFrames(t, p, []Frame[testFrame]{{testFrame{"frame 2"}, 0, 150}})
		expectFrames(t, p, []Frame[testFrame]{{testFrame{"frame 2"}, 0, 150}})
		mustAddFrames(t, p, []Frame[testFrame]{{testFrame{"frame 3"}, 0, 200}})
		expectFrames(t, p, []Frame[testFrame]{{testFrame{"frame 3"}, 0, 200}})
		mustAddFrames(t, p, []Frame[testFrame]{{testFrame{"frame 4"}, 0, 250}})
		expectFrames(t, p, []Frame[testFrame]{{testFrame{"frame 4"}, 0, 250}})
	})

	t.Run("overflow", func(t *testing.T) {
		p, _ := startPlayer(t, 2)
		go mustAddFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
		expectFrames(t, p, []Frame[testFrame]{
			{testFrame{"frame 1"}, 0, 100},
			{testFrame{"frame 2"}, 0, 150},
			{testFrame{"frame 3"}, 0, 200},
			{testFrame{"frame 4"}, 0, 250},
		})
	})

	t.Run("no_frames", func(t *testing.T) {
		p, _ := startPlayer(t, 10)
		select {
		case <-time.After(100 * time.Millisecond):
		case frame := <-p.C:
			t.Errorf("got unexpected frame: %v", frame)
		}
	})
}

type testPlayer[Image any] struct {
	*Player[Image]
	ctx context.Context
}

func startPlayer(t *testing.T, maxFrames int) (player *testPlayer[testFrame], done func(error)) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	p := NewPlayerWithSize[testFrame](maxFrames)

	errCh := make(chan error, 1)
	go func() { errCh <- p.Run(ctx) }()

	return &testPlayer[testFrame]{p, ctx}, func(expectErr error) {
		if ctx.Err() != nil {
			return
		}

		cancel()

		if expectErr == nil {
			expectErr = context.Canceled
		}
		if err := <-errCh; err != expectErr {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func expectFrames(t *testing.T, p *testPlayer[testFrame], frames []Frame[testFrame]) {
	t.Helper()

	ctx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	var lastTime time.Time
	var frame Frame[testFrame]

	for len(frames) > 0 {
		select {
		case <-ctx.Done():
			t.Error("timed out")
			return
		case frame = <-p.C:
		}

		now := time.Now()
		if !lastTime.IsZero() {
			offset := now.Sub(lastTime)
			latency := intmath.Abs(offset - frame.Duration())
			t.Logf("+%v (jitter %v)", offset, latency)
			if latency > frameTimeMargin {
				t.Error("frame duration was off by", latency)
			}
		}
		lastTime = now

		t.Log("got", frame)

		assert.Equal(t, frames[0], frame)
		frames = frames[1:]
	}
}

func mustAddFrames(t *testing.T, p *testPlayer[testFrame], frames []Frame[testFrame]) {
	t.Helper()
	if err := p.AddFrames(p.ctx, frames); err != nil {
		t.Error(err)
	}
}
