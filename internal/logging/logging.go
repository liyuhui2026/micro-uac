package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/liyuhui/micro-uac/internal/config"
	"github.com/rs/zerolog"
)

func New(cfg config.LogConfig) (zerolog.Logger, func() error, error) {
	level, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("parse log level: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.File), 0o755); err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("create log dir: %w", err)
	}

	file, err := os.OpenFile(cfg.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return zerolog.Logger{}, nil, fmt.Errorf("open log file: %w", err)
	}

	var writer io.Writer = file
	if cfg.AlsoStdout {
		writer = io.MultiWriter(file, os.Stdout)
	}

	logger := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	return logger, file.Close, nil
}
