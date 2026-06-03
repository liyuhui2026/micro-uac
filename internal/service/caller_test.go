package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/liyuhui/micro-uac/internal/config"
	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/media"
	"github.com/liyuhui/micro-uac/internal/sdp"
	"github.com/rs/zerolog"
)

type fakeDialog struct {
	remote    domain.RemoteMedia
	waitCh    chan error
	hangupErr error
}

func (f *fakeDialog) CallID() string                   { return "dialog-id" }
func (f *fakeDialog) SIPCallID() string                { return "sip-call-id" }
func (f *fakeDialog) RemoteMedia() domain.RemoteMedia  { return f.remote }
func (f *fakeDialog) Wait(ctx context.Context) error   { return <-f.waitCh }
func (f *fakeDialog) Hangup(ctx context.Context) error { return f.hangupErr }

type fakeSIP struct {
	dialog domain.EstablishedCall
	err    error
}

func (f *fakeSIP) Dial(ctx context.Context, req domain.CallRequest, offer sdp.Offer) (domain.EstablishedCall, error) {
	return f.dialog, f.err
}

type fakeStream struct {
	err error
}

func (f *fakeStream) Stream(ctx context.Context, localPort int, remote domain.RemoteMedia, source *media.Source) error {
	return f.err
}

func TestCallerValidation(t *testing.T) {
	caller := NewCaller(config.Default(), zerolog.Nop(), &fakeSIP{}, &fakeStream{})
	_, err := caller.Dial(context.Background(), domain.CallRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestCallerStreamFailure(t *testing.T) {
	cfg := config.Default()
	dialog := &fakeDialog{
		remote: domain.RemoteMedia{Host: "127.0.0.1", Port: 9000, PayloadType: 0, Codec: domain.CodecPCMU},
		waitCh: make(chan error, 1),
	}
	dialog.waitCh <- context.Canceled
	caller := NewCaller(cfg, zerolog.Nop(), &fakeSIP{dialog: dialog}, &fakeStream{err: errors.New("boom")})
	_, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "sip:b@example.com",
		RequestURI: "sip:b@example.com",
		AudioFile:  "missing.pcm",
	})
	if err == nil {
		t.Fatal("expected failure")
	}
}

func TestHostFromListenAddr(t *testing.T) {
	host, err := hostFromListenAddr("0.0.0.0:5060")
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("unexpected host %s", host)
	}
}

func TestCallerUsesDefaultAudioFileFromConfig(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "tone.pcm")
	raw := make([]byte, 320*2)
	if err := os.WriteFile(audioPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Media.DefaultAudioFile = audioPath
	dialog := &fakeDialog{
		remote: domain.RemoteMedia{Host: "127.0.0.1", Port: 9000, PayloadType: 0, Codec: domain.CodecPCMU},
		waitCh: make(chan error, 1),
	}
	dialog.waitCh <- context.Canceled
	caller := NewCaller(cfg, zerolog.Nop(), &fakeSIP{dialog: dialog}, &fakeStream{})

	res, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "sip:b@example.com",
		RequestURI: "sip:b@example.com",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if res.State != domain.CallStateCompleted {
		t.Fatalf("unexpected state: %s", res.State)
	}
}

func TestCallerRemoteHangupDoesNotFailOnLocalHangup(t *testing.T) {
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "tone.pcm")
	raw := make([]byte, 320*2)
	if err := os.WriteFile(audioPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Media.DefaultAudioFile = audioPath
	dialog := &fakeDialog{
		remote:    domain.RemoteMedia{Host: "127.0.0.1", Port: 9000, PayloadType: 0, Codec: domain.CodecPCMU},
		waitCh:    make(chan error, 1),
		hangupErr: errors.New("send bye: Timer_B timed out. transaction timeout"),
	}
	dialog.waitCh <- context.Canceled
	caller := NewCaller(cfg, zerolog.Nop(), &fakeSIP{dialog: dialog}, &fakeStream{})

	res, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "sip:b@example.com",
		RequestURI: "sip:b@example.com",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if res.State != domain.CallStateCompleted {
		t.Fatalf("unexpected state: %s", res.State)
	}
}
