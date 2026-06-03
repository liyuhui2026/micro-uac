package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/liyuhui/micro-uac/internal/config"
	"github.com/liyuhui/micro-uac/internal/httpapi"
	"github.com/liyuhui/micro-uac/internal/logging"
	"github.com/liyuhui/micro-uac/internal/rtp"
	"github.com/liyuhui/micro-uac/internal/service"
	sipclient "github.com/liyuhui/micro-uac/internal/sip"
	"github.com/liyuhui/micro-uac/internal/task"
	"github.com/rs/zerolog"
)

type HTTPServer struct {
	httpServer *http.Server
	logger     zerolog.Logger
}

func NewCLI(configPath string) (*service.Caller, func(), error) {
	cfg, logger, sip, cleanup, err := bootstrap(configPath)
	if err != nil {
		return nil, nil, err
	}
	caller := service.NewCaller(cfg, logger, sip, rtp.NewSender(logger))
	return caller, cleanup, nil
}

func NewServer(configPath string) (*HTTPServer, func(), error) {
	cfg, logger, sip, cleanup, err := bootstrap(configPath)
	if err != nil {
		return nil, nil, err
	}

	caller := service.NewCaller(cfg, logger, sip, rtp.NewSender(logger))
	manager := task.NewManager(caller)
	handler := httpapi.NewHandler(manager)
	server := &http.Server{
		Addr:    cfg.HTTP.ListenAddr,
		Handler: handler.Routes(),
	}

	return &HTTPServer{
		httpServer: server,
		logger:     logger,
	}, cleanup, nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info().Str("listen_addr", s.httpServer.Addr).Msg("http server started")
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		return s.httpServer.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func bootstrap(configPath string) (config.Config, zerolog.Logger, *sipclient.Client, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, zerolog.Logger{}, nil, nil, err
	}
	logger, closeLog, err := logging.New(cfg.Log)
	if err != nil {
		return config.Config{}, zerolog.Logger{}, nil, nil, err
	}

	sip, closeSIP, err := sipclient.NewClient(cfg.SIP, logger.With().Str("component", "sip").Logger())
	if err != nil {
		_ = closeLog()
		return config.Config{}, zerolog.Logger{}, nil, nil, err
	}

	cleanup := func() {
		if err := closeSIP(); err != nil {
			logger.Error().Err(err).Msg("close sip ua")
		}
		if err := closeLog(); err != nil {
			fmt.Printf("close log file: %v\n", err)
		}
	}

	return cfg, logger, sip, cleanup, nil
}
