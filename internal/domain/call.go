package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Codec string

const (
	CodecPCMU Codec = "pcmu"
	CodecPCMA Codec = "pcma"
)

func (c Codec) Validate() error {
	switch strings.ToLower(string(c)) {
	case string(CodecPCMU), string(CodecPCMA):
		return nil
	default:
		return errors.New("unsupported codec")
	}
}

func (c Codec) Canonical() Codec {
	switch strings.ToLower(string(c)) {
	case string(CodecPCMA):
		return CodecPCMA
	default:
		return CodecPCMU
	}
}

type CallState string

const (
	CallStateCreated     CallState = "created"
	CallStateDialing     CallState = "dialing"
	CallStateRinging     CallState = "ringing"
	CallStateAnswered    CallState = "answered"
	CallStateStreaming   CallState = "streaming"
	CallStateTerminating CallState = "terminating"
	CallStateCompleted   CallState = "completed"
	CallStateFailed      CallState = "failed"
)

type CallRequest struct {
	From       string `json:"from"`
	To         string `json:"to"`
	RequestURI string `json:"request_uri"`
	AudioFile  string `json:"audio_file"`
	Codec      Codec  `json:"codec"`
	FrameMS    int    `json:"frame_ms"`
}

type CallResult struct {
	CallID    string    `json:"call_id"`
	SIPCallID string    `json:"sip_call_id,omitempty"`
	State     CallState `json:"state"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

type RemoteMedia struct {
	Host        string
	Port        int
	PayloadType uint8
	Codec       Codec
}

type EstablishedCall interface {
	CallID() string
	SIPCallID() string
	RemoteMedia() RemoteMedia
	Wait(ctx context.Context) error
	Hangup(ctx context.Context) error
}

type Caller interface {
	Dial(ctx context.Context, req CallRequest) (CallResult, error)
}
