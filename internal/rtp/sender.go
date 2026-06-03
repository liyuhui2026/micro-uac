package rtp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/media"
	pionrtp "github.com/pion/rtp"
	"github.com/rs/zerolog"
)

type Sender struct {
	logger zerolog.Logger
	dialer func(network string, laddr, raddr *net.UDPAddr) (*net.UDPConn, error)
}

func NewSender(logger zerolog.Logger) *Sender {
	return &Sender{
		logger: logger,
		dialer: func(network string, laddr, raddr *net.UDPAddr) (*net.UDPConn, error) {
			return net.DialUDP(network, laddr, raddr)
		},
	}
}

func (s *Sender) Stream(ctx context.Context, localPort int, remote domain.RemoteMedia, source *media.Source) error {
	remoteAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", remote.Host, remote.Port))
	if err != nil {
		return fmt.Errorf("resolve rtp target: %w", err)
	}
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: localPort}
	conn, err := s.dialer("udp", localAddr, remoteAddr)
	if err != nil {
		return fmt.Errorf("dial rtp target: %w", err)
	}
	defer conn.Close()

	s.logger.Info().
		Int("local_port", localPort).
		Str("remote_host", remote.Host).
		Int("remote_port", remote.Port).
		Uint8("payload_type", remote.PayloadType).
		Str("codec", string(remote.Codec)).
		Msg("starting rtp stream")

	frameDuration := time.Duration(source.FrameSamples()) * time.Second / 8000
	timestampStep := uint32(source.FrameSamples())
	sequence := uint16(1)
	timestamp := uint32(0)
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()
	framesSent := 0

	for {
		payload, ok, err := source.NextFrame()
		if err != nil {
			return fmt.Errorf("read audio frame: %w", err)
		}
		if !ok {
			return nil
		}

		packet := &pionrtp.Packet{
			Header: pionrtp.Header{
				Version:        2,
				PayloadType:    remote.PayloadType,
				SequenceNumber: sequence,
				Timestamp:      timestamp,
				SSRC:           1,
			},
			Payload: payload,
		}
		raw, err := packet.Marshal()
		if err != nil {
			return fmt.Errorf("marshal rtp packet: %w", err)
		}
		if _, err := conn.Write(raw); err != nil {
			return fmt.Errorf("send rtp packet: %w", err)
		}

		framesSent++
		if framesSent == 1 || framesSent%50 == 0 {
			s.logger.Info().
				Int("frames_sent", framesSent).
				Uint16("sequence", sequence).
				Uint32("timestamp", timestamp).
				Int("payload_bytes", len(payload)).
				Msg("rtp frame sent")
		}

		sequence++
		timestamp += timestampStep

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
