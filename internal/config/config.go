package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"

	"github.com/liyuhui/micro-uac/internal/domain"
)

type Config struct {
	SIP   SIPConfig   `json:"sip"`
	Log   LogConfig   `json:"log"`
	Media MediaConfig `json:"media"`
	HTTP  HTTPConfig  `json:"http"`
}

type SIPConfig struct {
	ListenAddr string `json:"listen_addr"`
	ExternalIP string `json:"external_ip"`
	UserAgent  string `json:"user_agent"`
}

type LogConfig struct {
	Level      string `json:"level"`
	File       string `json:"file"`
	AlsoStdout bool   `json:"also_stdout"`
}

type MediaConfig struct {
	DefaultAudioFile string       `json:"default_audio_file"`
	DefaultCodec   domain.Codec `json:"default_codec"`
	DefaultFrameMS int          `json:"default_frame_ms"`
}

type HTTPConfig struct {
	ListenAddr string `json:"listen_addr"`
}

func Default() Config {
	return Config{
		SIP: SIPConfig{
			ListenAddr: "0.0.0.0:5060",
			ExternalIP: "127.0.0.1",
			UserAgent:  "micro-uac/1.0",
		},
		Log: LogConfig{
			Level: "info",
			File:  ".runtime/micro-uac.log",
		},
		Media: MediaConfig{
			DefaultAudioFile: "",
			DefaultCodec:   domain.CodecPCMU,
			DefaultFrameMS: 20,
		},
		HTTP: HTTPConfig{
			ListenAddr: ":8080",
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = "config.json"
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.SIP.ListenAddr == "" {
		return errors.New("sip.listen_addr is required")
	}
	if c.SIP.ExternalIP == "" {
		host, _, err := splitHostPort(c.SIP.ListenAddr)
		if err != nil {
			return err
		}
		if host == "" {
			return errors.New("sip.external_ip is required when sip.listen_addr host is empty")
		}
		if ip, err := netip.ParseAddr(host); err == nil && ip.IsUnspecified() {
			return fmt.Errorf("sip.external_ip is required when sip.listen_addr uses unspecified host %q", host)
		}
	}
	if c.Log.File == "" {
		return errors.New("log.file is required")
	}
	if err := c.Media.DefaultCodec.Canonical().Validate(); err != nil {
		return fmt.Errorf("media.default_codec: %w", err)
	}
	if c.Media.DefaultFrameMS <= 0 {
		return errors.New("media.default_frame_ms must be greater than 0")
	}
	if c.HTTP.ListenAddr == "" {
		return errors.New("http.listen_addr is required")
	}
	return nil
}

func (c Config) ResolveLogPath() string {
	return filepath.Clean(c.Log.File)
}

func splitHostPort(addr string) (string, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", "", fmt.Errorf("parse sip.listen_addr: %w", err)
	}
	return host, port, nil
}
