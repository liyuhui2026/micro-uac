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
	dialog  domain.EstablishedCall
	err     error
	lastReq domain.CallRequest
}

func (f *fakeSIP) Dial(ctx context.Context, req domain.CallRequest, offer sdp.Offer) (domain.EstablishedCall, error) {
	f.lastReq = req
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
		RequestURI: "sip:b@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
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
		RequestURI: "sip:b@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
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
		RequestURI: "sip:b@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if res.State != domain.CallStateCompleted {
		t.Fatalf("unexpected state: %s", res.State)
	}
}

func TestCallerUsesConfigFSAddrByDefault(t *testing.T) {
	cfg := config.Default()
	cfg.FSAddr = "192.0.2.10:5060"
	dialog := &fakeDialog{
		remote: domain.RemoteMedia{Host: "127.0.0.1", Port: 9000, PayloadType: 0, Codec: domain.CodecPCMU},
		waitCh: make(chan error, 1),
	}
	dialog.waitCh <- context.Canceled
	fakeSIP := &fakeSIP{dialog: dialog}
	caller := NewCaller(cfg, zerolog.Nop(), fakeSIP, &fakeStream{})

	_, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "sip:b@example.com",
		RequestURI: "sip:b@198.51.100.20:5080",
		AudioFile:  filepath.Join(t.TempDir(), "missing.wav"),
	})
	if err == nil {
		t.Fatal("expected audio file error")
	}
}

func TestCallerUsesRequestURIAsDefaultLineAddr(t *testing.T) {
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
	fakeSIP := &fakeSIP{dialog: dialog}
	caller := NewCaller(cfg, zerolog.Nop(), fakeSIP, &fakeStream{})

	_, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "<sip:1012@example.com>",
		RequestURI: "sip:1012@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if fakeSIP.lastReq.LineAddr != "198.51.100.20:5080" {
		t.Fatalf("unexpected line_addr: %q", fakeSIP.lastReq.LineAddr)
	}
	if fakeSIP.lastReq.TargetURI != "sip:1012@198.51.100.20:5080" {
		t.Fatalf("unexpected target_uri default: %q", fakeSIP.lastReq.TargetURI)
	}
}

func TestCallerRejectsInvalidLineAddr(t *testing.T) {
	caller := NewCaller(config.Default(), zerolog.Nop(), &fakeSIP{}, &fakeStream{})
	_, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "<sip:1012@example.com>",
		RequestURI: "sip:1012@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
		LineAddr:   "bad-line-addr",
	})
	if err == nil {
		t.Fatal("expected invalid line_addr error")
	}
}

func TestCallerAcceptsIndependentTargetURI(t *testing.T) {
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
	fakeSIP := &fakeSIP{dialog: dialog}
	caller := NewCaller(cfg, zerolog.Nop(), fakeSIP, &fakeStream{})

	_, err := caller.Dial(context.Background(), domain.CallRequest{
		From:       "sip:a@example.com",
		To:         "<sip:1012@example.com>",
		RequestURI: "sip:3001@198.51.100.20:5080",
		FSAddr:     "127.0.0.1:5060",
		LineAddr:   "198.51.100.20:5080",
		TargetURI:  "sip:2001@192.0.2.50:5090",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if fakeSIP.lastReq.TargetURI != "sip:2001@192.0.2.50:5090" {
		t.Fatalf("unexpected target_uri: %q", fakeSIP.lastReq.TargetURI)
	}
}
