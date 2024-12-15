package upgrader

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type GracefulUpgrader struct {
	logger *zap.Logger
	ln     net.Listener
	pid    int
	ppid   int
}

func NewGracefulUpgrader(logger *zap.Logger) *GracefulUpgrader {
	return &GracefulUpgrader{
		logger: logger,
		pid:    os.Getpid(),
		ppid:   os.Getppid(),
	}
}

// Listen 创建或继承 listener
func (g *GracefulUpgrader) Listen(network, address string) (net.Listener, error) {
	// 检查是否从父进程继承了文件描述符
	if os.Getenv("GRACEFUL_RESTART") == "true" {
		g.logger.Info("Inheriting listener from parent process",
			zap.Int("pid", g.pid),
			zap.Int("ppid", g.ppid),
		)

		// 获取继承的文件描述符
		f := os.NewFile(3, "") // 3 是第一个可用的文件描述符
		ln, err := net.FileListener(f)
		if err != nil {
			return nil, fmt.Errorf("failed to inherit listener: %w", err)
		}
		f.Close()
		g.ln = ln
		return ln, nil
	}

	// 创建新的 listener
	ln, err := net.Listen(network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	g.ln = ln
	return ln, nil
}

// Reload 执行平滑重启
func (g *GracefulUpgrader) Reload() error {
	g.logger.Info("Starting graceful reload",
		zap.Int("old_pid", g.pid),
	)

	// 获取当前可执行文件路径
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// 将 listener 转换为文件描述符
	listenerFile, err := g.ln.(*net.TCPListener).File()
	if err != nil {
		return fmt.Errorf("failed to get listener file: %w", err)
	}
	defer listenerFile.Close()

	// 准备环境变量
	env := append(os.Environ(), "GRACEFUL_RESTART=true")

	// 创建新进程
	process, err := os.StartProcess(executable, os.Args, &os.ProcAttr{
		Env:   env,
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr, listenerFile},
	})
	if err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}

	g.logger.Info("Started new process",
		zap.Int("new_pid", process.Pid),
	)

	return nil
}

// WaitForSignal 等待信号并处理
func (g *GracefulUpgrader) WaitForSignal(server interface {
	Shutdown(context.Context) error
}) error {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	for {
		sig := <-signalChan
		switch sig {
		case syscall.SIGHUP:
			// 收到 HUP 信号，执行平滑重启
			if err := g.Reload(); err != nil {
				g.logger.Error("Failed to reload", zap.Error(err))
				continue
			}

			// 等待新进程启动后优雅关闭当前进程
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				g.logger.Error("Failed to shutdown", zap.Error(err))
				return err
			}

			g.logger.Info("Graceful reload completed", zap.Int("pid", g.pid))
			return nil

		case syscall.SIGTERM, syscall.SIGINT:
			// 收到终止信号，执行优雅关闭
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				g.logger.Error("Failed to shutdown", zap.Error(err))
				return err
			}

			g.logger.Info("Graceful shutdown completed", zap.Int("pid", g.pid))
			return nil
		}
	}
}
