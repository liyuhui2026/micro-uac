package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config valid, got %v", err)
	}
}

func TestInvalidFrameMS(t *testing.T) {
	cfg := Default()
	cfg.Media.DefaultFrameMS = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestExternalIPRequiredForUnspecifiedSIPHost(t *testing.T) {
	cfg := Default()
	cfg.SIP.ListenAddr = "0.0.0.0:5060"
	cfg.SIP.ExternalIP = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestLoadJSONConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	raw := `{
		"sip": {
			"listen_addr": "127.0.0.1:5060",
			"external_ip": "203.0.113.10",
			"user_agent": "test-uac/1.0"
		},
		"log": {
			"level": "debug",
			"file": "logs/test.log",
			"also_stdout": true
		},
		"media": {
			"default_audio_file": "audio/test.wav",
			"default_codec": "pcma",
			"default_frame_ms": 30
		},
		"http": {
			"listen_addr": ":18080"
		}
	}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.SIP.ExternalIP != "203.0.113.10" {
		t.Fatalf("unexpected external ip: %s", cfg.SIP.ExternalIP)
	}
	if cfg.Media.DefaultCodec != "pcma" {
		t.Fatalf("unexpected codec: %s", cfg.Media.DefaultCodec)
	}
	if cfg.Media.DefaultAudioFile != "audio/test.wav" {
		t.Fatalf("unexpected default audio file: %s", cfg.Media.DefaultAudioFile)
	}
	if cfg.HTTP.ListenAddr != ":18080" {
		t.Fatalf("unexpected http listen addr: %s", cfg.HTTP.ListenAddr)
	}
}
