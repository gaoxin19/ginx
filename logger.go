package ginx

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogConfig 日志配置
type LogConfig struct {
	Level      string
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
	LocalTime  bool
	Console    bool
}

// NewLogger 创建日志实例
func NewLogger(conf *LogConfig) (*zap.Logger, error) {
	if conf.Filename != "" {
		if err := os.MkdirAll(filepath.Dir(conf.Filename), 0744); err != nil {
			return nil, fmt.Errorf("can't create log directory: %w", err)
		}
	}

	level, err := zapcore.ParseLevel(conf.Level)
	if err != nil {
		return nil, fmt.Errorf("parse log level error: %w", err)
	}

	cores := make([]zapcore.Core, 0)
	encoderConfig := newEncoderConfig()

	// 文件输出
	if conf.Filename != "" {
		fileWriter := zapcore.AddSync(&lumberjack.Logger{
			Filename:   conf.Filename,
			MaxSize:    conf.MaxSize,
			MaxBackups: conf.MaxBackups,
			MaxAge:     conf.MaxAge,
			Compress:   conf.Compress,
			LocalTime:  conf.LocalTime,
		})

		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			fileWriter,
			level,
		))
	}

	// 控制台输出
	if conf.Console {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.Lock(os.Stdout),
			level,
		))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return logger, nil
}

func newEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

var (
	defaultLogger *zap.Logger = func() *zap.Logger {
		// 创建默认的命令行日志配置
		conf := &LogConfig{
			Level:   "info",
			Console: true,
		}
		logger, err := NewLogger(conf)
		if err != nil {
			// 如果出现错误，创建一个基础的开发模式logger
			logger, _ = zap.NewDevelopment()
		}
		return logger
	}()
)

// SetLogger 设置全局日志实例
func SetLogger(logger *zap.Logger) {
	defaultLogger = logger
}

// L 获取全局日志实例
func L() *zap.Logger {
	return defaultLogger
}
