package logging

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New returns a production zap logger. If dev is true, uses the
// human-readable console encoder with DEBUG level.
func New(dev bool) (*zap.Logger, error) {
	if dev {
		cfg := zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		return cfg.Build()
	}
	return zap.NewProduction()
}
