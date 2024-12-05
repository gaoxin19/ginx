package upgrader

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudflare/tableflip"
	"go.uber.org/zap"
)

// Upgrader 优雅重启接口
type Upgrader interface {
	Listen(network, addr string) (net.Listener, error)
	Ready() error
	Exit() <-chan struct{}
	Stop()
	WatchSignal()
}

type upgrader struct {
	upg    *tableflip.Upgrader
	logger *zap.Logger
}

// New 创建新的升级器
func New(logger *zap.Logger) (Upgrader, error) {
	upg, err := tableflip.New(tableflip.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create upgrader: %w", err)
	}

	return &upgrader{
		upg:    upg,
		logger: logger,
	}, nil
}

func (u *upgrader) Listen(network, addr string) (net.Listener, error) {
	ln, err := u.upg.Listen(network, addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	return ln, nil
}

func (u *upgrader) WatchSignal() {
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGHUP)
		for range sig {
			err := u.upg.Upgrade()
			if err != nil {
				u.logger.Error("Upgrade failed", zap.Error(err))
			}
		}
	}()
}

func (u *upgrader) Ready() error {
	return u.upg.Ready()
}

func (u *upgrader) Exit() <-chan struct{} {
	return u.upg.Exit()
}

func (u *upgrader) Stop() {
	u.upg.Stop()
}
