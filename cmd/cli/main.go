package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/liyuhui/micro-uac/internal/app"
	"github.com/liyuhui/micro-uac/internal/domain"
)

func main() {
	var (
		configPath = flag.String("config", "config.json", "path to JSON config file")
		from       = flag.String("from", "", "SIP From user or URI")
		to         = flag.String("to", "", "SIP To user or URI")
		requestURI = flag.String("request-uri", "", "destination SIP request URI")
		audioFile  = flag.String("audio-file", "", "path to local WAV or PCM audio file")
		codec      = flag.String("codec", "", "audio codec: pcmu or pcma")
		frameMS    = flag.Int("frame-ms", 0, "RTP frame duration in milliseconds")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner, cleanup, err := app.NewCLI(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()

	req := domain.CallRequest{
		From:       *from,
		To:         *to,
		RequestURI: *requestURI,
		AudioFile:  *audioFile,
	}
	if *codec != "" {
		req.Codec = domain.Codec(*codec)
	}
	if *frameMS > 0 {
		req.FrameMS = *frameMS
	}

	res, err := runner.Dial(ctx, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "call failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("call_id=%s state=%s sip_call_id=%s\n", res.CallID, res.State, res.SIPCallID)
}
