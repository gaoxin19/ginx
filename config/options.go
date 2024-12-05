package config

import (
	"time"
)

// Options 引擎配置选项
type Options struct {
	// 服务配置
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// 日志配置
	Logger *LogOptions

	// 中间件配置
	EnableRecovery bool
	EnableLogger   bool
}

// LogOptions 日志配置选项
type LogOptions struct {
	Level      string
	Filename   string
	MaxSize    int
	MaxAge     int
	MaxBackups int
	Compress   bool
	LocalTime  bool
	Console    bool
}

// DefaultOptions 返回默认配置
func DefaultOptions() *Options {
	return &Options{
		Port:         8080,
		ReadTimeout:  time.Second * 30,
		WriteTimeout: time.Second * 30,
		Logger: &LogOptions{
			Level:      "info",
			MaxSize:    100,
			MaxAge:     30,
			MaxBackups: 30,
			Compress:   true,
			LocalTime:  true,
			Console:    true,
		},
		EnableRecovery: true,
		EnableLogger:   true,
	}
}
