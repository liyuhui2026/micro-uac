package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/liyuhui/micro-uac/internal/domain"
)

func TestSourcePCMFrames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.pcm")
	raw := make([]byte, 320*2)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	source, err := NewSource(path, domain.CodecPCMU, 20)
	if err != nil {
		t.Fatalf("new source: %v", err)
	}

	frame, ok, err := source.NextFrame()
	if err != nil {
		t.Fatalf("next frame: %v", err)
	}
	if !ok {
		t.Fatal("expected frame")
	}
	if len(frame) != 160 {
		t.Fatalf("expected 160 encoded samples, got %d", len(frame))
	}
}

func TestInvalidExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tone.mp3")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSource(path, domain.CodecPCMU, 20); err == nil {
		t.Fatal("expected unsupported extension error")
	}
}
