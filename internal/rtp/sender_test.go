package rtp

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/liyuhui/micro-uac/internal/domain"
	"github.com/liyuhui/micro-uac/internal/media"
	pionrtp "github.com/pion/rtp"
	"github.com/rs/zerolog"
)

func TestSenderSequenceAndTimestamp(t *testing.T) {
	logger := zerolog.Nop()
	sender := NewSender(logger)

	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	dir := t.TempDir()
	path := dir + "/tone.pcm"
	raw := make([]byte, 320*2)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	source, err := media.NewSource(path, domain.CodecPCMU, 20)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		addr := server.LocalAddr().(*net.UDPAddr)
		errCh <- sender.Stream(ctx, 0, domain.RemoteMedia{
			Host:        addr.IP.String(),
			Port:        addr.Port,
			PayloadType: 0,
			Codec:       domain.CodecPCMU,
		}, source)
	}()

	buffer := make([]byte, 1500)
	packets := make([]*pionrtp.Packet, 0, 2)
	for len(packets) < 2 {
		_ = server.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := server.ReadFrom(buffer)
		if err != nil {
			t.Fatal(err)
		}
		packet := &pionrtp.Packet{}
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			t.Fatal(err)
		}
		packets = append(packets, packet)
	}

	if packets[0].SequenceNumber+1 != packets[1].SequenceNumber {
		t.Fatalf("unexpected sequence increment: %d -> %d", packets[0].SequenceNumber, packets[1].SequenceNumber)
	}
	if packets[0].Timestamp+160 != packets[1].Timestamp {
		t.Fatalf("unexpected timestamp increment: %d -> %d", packets[0].Timestamp, packets[1].Timestamp)
	}
}
