package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/liyuhui/micro-uac/internal/config"
	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/media"
	"github.com/liyuhui/micro-uac/internal/sdp"
	"github.com/rs/zerolog"
)

type SIPDialer interface {
	Dial(ctx context.Context, req domain.CallRequest, offer sdp.Offer) (domain.EstablishedCall, error)
}

type RTPStreamer interface {
	Stream(ctx context.Context, localPort int, remote domain.RemoteMedia, source *media.Source) error
}

type Caller struct {
	cfg    config.Config
	logger zerolog.Logger
	sip    SIPDialer
	stream RTPStreamer
}

func NewCaller(cfg config.Config, logger zerolog.Logger, sip SIPDialer, stream RTPStreamer) *Caller {
	return &Caller{
		cfg:    cfg,
		logger: logger,
		sip:    sip,
		stream: stream,
	}
}

func (c *Caller) Dial(ctx context.Context, req domain.CallRequest) (domain.CallResult, error) {
	callID := uuid.NewString()
	req, err := c.normalize(req)
	if err != nil {
		return domain.CallResult{
			CallID:    callID,
			State:     domain.CallStateFailed,
			StartedAt: time.Now(),
			EndedAt:   time.Now(),
			Reason:    err.Error(),
		}, err
	}

	logger := c.logger.With().Str("call_id", callID).Logger()
	startedAt := time.Now()
	result := domain.CallResult{
		CallID:    callID,
		State:     domain.CallStateDialing,
		StartedAt: startedAt,
	}

	logger.Info().
		Str("from", req.From).
		Str("to", req.To).
		Str("request_uri", req.RequestURI).
		Str("audio_file", req.AudioFile).
		Str("codec", string(req.Codec)).
		Int("frame_ms", req.FrameMS).
		Msg("starting outbound call")

	rtpPort, err := reserveUDPPort()
	if err != nil {
		return c.fail(result, logger, fmt.Errorf("allocate rtp port: %w", err))
	}

	localSDPIP := c.cfg.SIP.ExternalIP
	if localSDPIP == "" {
		localSDPIP, err = hostFromListenAddr(c.cfg.SIP.ListenAddr)
		if err != nil {
			return c.fail(result, logger, err)
		}
	}

	offer := sdp.BuildOffer(localSDPIP, rtpPort, req.Codec)
	dialog, err := c.sip.Dial(ctx, req, offer)
	if err != nil {
		return c.fail(result, logger, err)
	}
	result.SIPCallID = dialog.SIPCallID()
	result.State = domain.CallStateAnswered

	logger.Info().
		Str("sip_call_id", dialog.SIPCallID()).
		Str("remote_host", dialog.RemoteMedia().Host).
		Int("remote_port", dialog.RemoteMedia().Port).
		Str("remote_codec", string(dialog.RemoteMedia().Codec)).
		Msg("call answered")

	source, err := media.NewSource(req.AudioFile, dialog.RemoteMedia().Codec, req.FrameMS)
	if err != nil {
		_ = dialog.Hangup(context.Background())
		return c.fail(result, logger, err)
	}

	result.State = domain.CallStateStreaming
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	streamErrCh := make(chan error, 1)
	go func() {
		streamErrCh <- c.stream.Stream(streamCtx, rtpPort, dialog.RemoteMedia(), source)
	}()

	select {
	case err := <-streamErrCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			_ = dialog.Hangup(context.Background())
			return c.fail(result, logger, err)
		}
		result.State = domain.CallStateTerminating
		logger.Info().Msg("audio streaming finished; hanging up call")
		hangupCtx, hangupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer hangupCancel()
		if err := dialog.Hangup(hangupCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "transaction timeout") {
				result.State = domain.CallStateCompleted
				result.EndedAt = time.Now()
				logger.Warn().Err(err).Msg("hangup timed out after audio finished; treating call as completed")
				return result, nil
			}
			return c.fail(result, logger, err)
		}
	case err := <-waitForDialog(ctx, dialog):
		cancel()
		if err != nil && !errors.Is(err, context.Canceled) {
			return c.fail(result, logger, err)
		}
		result.State = domain.CallStateCompleted
		result.EndedAt = time.Now()
		logger.Info().Msg("call completed by remote side")
		return result, nil
	case <-ctx.Done():
		cancel()
		_ = dialog.Hangup(context.Background())
		return c.fail(result, logger, ctx.Err())
	}

	if err := dialog.Wait(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return c.fail(result, logger, err)
	}

	result.State = domain.CallStateCompleted
	result.EndedAt = time.Now()
	logger.Info().Msg("call completed")
	return result, nil
}

func (c *Caller) normalize(req domain.CallRequest) (domain.CallRequest, error) {
	if req.From == "" || req.To == "" || req.RequestURI == "" {
		return req, errors.New("from, to and request_uri are required")
	}
	if req.AudioFile == "" {
		req.AudioFile = c.cfg.Media.DefaultAudioFile
	}
	if req.AudioFile == "" {
		return req, errors.New("audio_file is required")
	}
	if req.Codec == "" {
		req.Codec = c.cfg.Media.DefaultCodec
	}
	req.Codec = req.Codec.Canonical()
	if err := req.Codec.Validate(); err != nil {
		return req, err
	}
	if req.FrameMS == 0 {
		req.FrameMS = c.cfg.Media.DefaultFrameMS
	}
	if req.FrameMS <= 0 {
		return req, errors.New("frame_ms must be greater than 0")
	}
	return req, nil
}

func (c *Caller) fail(result domain.CallResult, logger zerolog.Logger, err error) (domain.CallResult, error) {
	result.State = domain.CallStateFailed
	result.EndedAt = time.Now()
	result.Reason = err.Error()
	logger.Error().Err(err).Msg("call failed")
	return result, err
}

func waitForDialog(ctx context.Context, dialog domain.EstablishedCall) <-chan error {
	ch := make(chan error, 1)
	go func() {
		ch <- dialog.Wait(ctx)
	}()
	return ch
}
