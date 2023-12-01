package christmasd

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gobwas/ws"
	"golang.org/x/sync/errgroup"
	"gopkg.in/typ.v4/sync2"
	"libdb.so/acm-christmas/internal/xcolor"
	"libdb.so/acm-christmas/lib/christmas/go/christmaspb"
)

// Config is the configuration for handling.
type Config struct {
	// Secret is the secret to use for the server.
	// The secret is used to authenticate the client.
	Secret string
}

// ServerOpts are options for a server.
type ServerOpts struct {
	// LEDController is the LED controller to use for the server.
	LEDController LEDController
	// Logger is the logger to use for the server.
	Logger *slog.Logger
	// HTTPUpgrader is the HTTP-to-Websocket upgrader to use for the server.
	HTTPUpgrader ws.HTTPUpgrader
}

// Server handles all HTTP requests for the server.
type Server struct {
	opts        ServerOpts
	cfg         atomic.Pointer[Config]
	connections sync2.Map[*Session, sessionControl]
}

type sessionControl struct {
	cancel context.CancelCauseFunc
}

// NewServer creates a new server.
func NewServer(cfg Config, opts ServerOpts) *Server {
	s := &Server{
		opts: opts,
	}
	s.cfg.Store(&cfg)
	return s
}

// KickAllConnections kicks all connections from the server.
// Optionally, a reason can be provided.
func (s *Server) KickAllConnections(reason string) {
	var err error
	if reason != "" {
		err = fmt.Errorf("kicked: %s", reason)
	} else {
		err = fmt.Errorf("kicked")
	}

	s.connections.Range(func(s *Session, ctrl sessionControl) bool {
		ctrl.cancel(err)
		return true
	})
}

// SetConfig sets the configuration for the server. All future connections will
// use the new configuration. Existing connections will continue to use the old
// configuration, unless they are kicked out.
func (s *Server) SetConfig(cfg Config) {
	s.cfg.Store(&cfg)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	session, err := SessionUpgrade(w, r, *s.cfg.Load(), s.opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithCancelCause(r.Context())
	s.connections.Store(session, sessionControl{cancel: cancel})

	if err := session.Start(ctx); err != nil {
		s.connections.Delete(session)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Session is a websocket session. It implements handling of messages from a
// single client.
type Session struct {
	ws     *websocketServer
	logger *slog.Logger
	opts   ServerOpts
	cfg    Config

	state atomic.Uint32
}

// SessionUpgrade upgrades an HTTP request to a websocket session.
func SessionUpgrade(w http.ResponseWriter, r *http.Request, cfg Config, opts ServerOpts) (*Session, error) {
	wsconn, _, _, err := opts.HTTPUpgrader.Upgrade(r, w)
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade HTTP: %w", err)
	}

	logger := opts.Logger.With("addr", wsconn.RemoteAddr())

	return &Session{
		ws:     newWebsocketServer(wsconn, logger),
		logger: logger,
		opts:   opts,
		cfg:    cfg,
	}, nil
}

// Start starts the server.
func (s *Session) Start(ctx context.Context) error {
	errg, ctx := errgroup.WithContext(ctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errg.Go(func() error {
		return s.ws.Start(ctx)
	})

	errg.Go(func() error {
		// Treat main loop errors as fatal and kill the connection,
		// but don't return it because it's not the caller's fault.
		if err := s.mainLoop(ctx); err != nil {
			return s.ws.SendError(ctx, err)
		}
		return nil
	})

	return errg.Wait()
}

var (
	errNotAuthenticated = fmt.Errorf("not authenticated")
	errInvalidSecret    = fmt.Errorf("invalid secret")
)

type sessionState = uint32

const (
	stateInitial sessionState = iota
	stateAuthenticated
)

func (s *Session) mainLoop(ctx context.Context) error {
	bufPbLED := make([]uint32, len(s.opts.LEDController.LEDs()))
	bufCtLED := make([]xcolor.RGB, len(s.opts.LEDController.LEDs()))

	for {
		select {
		case <-ctx.Done():
			return nil

		case msg := <-s.ws.Messages:
			if auth := msg.GetAuthenticate(); auth != nil {
				if auth.Secret != s.cfg.Secret {
					ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()

					s.ws.Send(ctx, &christmaspb.LEDServerMessage{
						Message: &christmaspb.LEDServerMessage_Authenticate{
							Authenticate: &christmaspb.AuthenticateResponse{
								Success: false,
							},
						},
					})

					return errInvalidSecret
				}

				if !s.state.CompareAndSwap(stateInitial, stateAuthenticated) {
					return fmt.Errorf("already authenticated")
				}

				s.ws.Send(ctx, &christmaspb.LEDServerMessage{
					Message: &christmaspb.LEDServerMessage_Authenticate{
						Authenticate: &christmaspb.AuthenticateResponse{
							Success: true,
						},
					},
				})

				s.logger.DebugContext(ctx,
					"new client authenticated")
				continue
			}

			if s.state.Load() != stateAuthenticated {
				return errNotAuthenticated
			}

			switch msg := msg.GetMessage().(type) {
			case *christmaspb.LEDClientMessage_GetLeds:
				ctLEDs := s.opts.LEDController.LEDs()
				for i, led := range ctLEDs {
					bufPbLED[i] = led.ToUint()
				}
				s.ws.Send(ctx, &christmaspb.LEDServerMessage{
					Message: &christmaspb.LEDServerMessage_GetLeds{
						GetLeds: &christmaspb.GetLEDsResponse{
							Leds: bufPbLED,
						},
					},
				})

			case *christmaspb.LEDClientMessage_SetLeds:
				pbLEDs := msg.SetLeds.GetLeds()
				if len(pbLEDs) != len(bufCtLED) {
					return fmt.Errorf("invalid number of LEDs: %d", len(pbLEDs))
				}
				for i, led := range pbLEDs {
					bufCtLED[i] = xcolor.RGBFromUint(led)
				}
				if err := s.opts.LEDController.SetLEDs(bufCtLED); err != nil {
					return fmt.Errorf("failed to set LEDs: %w", err)
				}

			case *christmaspb.LEDClientMessage_GetLedCanvasInfo:
				w, h := s.opts.LEDController.ImageSize()
				s.ws.Send(ctx, &christmaspb.LEDServerMessage{
					Message: &christmaspb.LEDServerMessage_GetLedCanvasInfo{
						GetLedCanvasInfo: &christmaspb.GetLEDCanvasInfoResponse{
							Width:  uint32(w),
							Height: uint32(h),
						},
					},
				})

			case *christmaspb.LEDClientMessage_SetLedCanvas:
				w, h := s.opts.LEDController.ImageSize()
				img := image.RGBA{
					Rect:   image.Rect(0, 0, w, h),
					Stride: w * 4,
					Pix:    msg.SetLedCanvas.GetPixels().GetPixels(),
				}
				if len(img.Pix) != w*h*4 {
					return fmt.Errorf("invalid image size")
				}
				if err := s.opts.LEDController.DrawImage(&img); err != nil {
					return fmt.Errorf("failed to draw image: %w", err)
				}
			}
		}
	}
}
