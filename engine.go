package ginx

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/gaoxin19/ginx/config"
	"github.com/gaoxin19/ginx/middleware"
	"github.com/gaoxin19/ginx/upgrader"
)

type Engine struct {
	*gin.Engine
	server            *http.Server
	upgrader          upgrader.Upgrader
	logger            *zap.Logger
	options           *config.Options
	shutdownCallbacks []func()
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

func (engine *Engine) GracefulServe(server *http.Server) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	errChan := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-quit:
		L().Info("Received shutdown signal, starting graceful shutdown...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		engine.executeShutdownCallbacks()

		if err := server.Shutdown(ctx); err != nil {
			L().Error("Server shutdown error", zap.Error(err))
			return fmt.Errorf("server shutdown error: %w", err)
		}

		L().Info("Server has been shutdown successfully")
		return nil

	case err := <-errChan:
		return fmt.Errorf("HTTP server error: %w", err)
	}
}

func (engine *Engine) RegisterOnShutdown(f func()) {
	engine.shutdownCallbacks = append(engine.shutdownCallbacks, f)
}

func (engine *Engine) executeShutdownCallbacks() {
	for _, cb := range engine.shutdownCallbacks {
		cb()
	}
}

func (e *Engine) GracefulRun() error {
	graceful := upgrader.NewGracefulUpgrader(e.logger)

	ln, err := graceful.Listen("tcp", fmt.Sprintf(":%d", e.options.Port))
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	e.logger.Info("Server is starting",
		zap.Int("port", e.options.Port),
		zap.Int("pid", os.Getpid()),
	)

	go func() {
		if err := e.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			e.logger.Error("Server error", zap.Error(err))
		}
	}()

	return graceful.WaitForSignal(e.server)
}
