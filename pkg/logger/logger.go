// Package logger provides the logger used by the service.
package logger

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config is the logger config
type Config struct {
	Dev       bool
	Verbosity int
}

// NewLogger returns a new logger
func NewLogger(conf Config) logr.Logger {
	var cfg zap.Config
	lvl := zap.NewAtomicLevelAt(zapcore.DPanicLevel - zapcore.Level(conf.Verbosity))
	if conf.Dev {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	cfg.Level = lvl

	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	return zapr.NewLogger(logger)
}
