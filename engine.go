package ginx

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/gaoxin19/ginx/config"
	"github.com/gaoxin19/ginx/middleware"
	"github.com/gaoxin19/ginx/upgrader"
)

type Engine struct {
	*gin.Engine
	server   *http.Server
	upgrader upgrader.Upgrader
	logger   *zap.Logger
	options  *config.Options
}

func New(opts *config.Options) (*Engine, error) {
	logger, err := NewLogger(&LogConfig{
		Level:      opts.Logger.Level,
		Filename:   opts.Logger.Filename,
		MaxSize:    opts.Logger.MaxSize,
		MaxAge:     opts.Logger.MaxAge,
		MaxBackups: opts.Logger.MaxBackups,
		Compress:   opts.Logger.Compress,
		LocalTime:  opts.Logger.LocalTime,
		Console:    opts.Logger.Console,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}
	SetLogger(logger)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	if opts.EnableRecovery {
		router.Use(middleware.Recovery(logger))
	}
	if opts.EnableLogger {
		router.Use(middleware.Logger(logger))
	}

	return &Engine{
		Engine: router,
		server: &http.Server{
			Handler:      router,
			ReadTimeout:  opts.ReadTimeout,
			WriteTimeout: opts.WriteTimeout,
		},
		logger:  logger,
		options: opts,
	}, nil
}

func (e *Engine) Run() error {
	upg, err := upgrader.New(e.logger)
	if err != nil {
		return fmt.Errorf("failed to create upgrader: %w", err)
	}
	e.upgrader = upg
	defer e.upgrader.Stop()
	ln, err := e.upgrader.Listen("tcp", fmt.Sprintf(":%d", e.options.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	if err := e.upgrader.Ready(); err != nil {
		return fmt.Errorf("failed to mark as ready: %w", err)
	}

	e.logger.Info("Server is starting", zap.Int("port", e.options.Port))

	go func() {
		if err := e.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			e.logger.Error("Server error", zap.Error(err))
			e.upgrader.Stop()
		}
	}()

	<-e.upgrader.Exit()
	return nil
}

func (e *Engine) Logger() *zap.Logger {
	return e.logger
}
