package logger

import (
	"errors"
	"os"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Initialize singleton logger.
func Initialize(level string) (*zap.SugaredLogger, error) {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, err
	}

	cfg := zap.NewDevelopmentConfig()
	cfg.Level = lvl
	cfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05")
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.MessageKey = "msg"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.DisableStacktrace = true

	zl, err := cfg.Build(
		zap.AddStacktrace(zapcore.FatalLevel),
		zap.AddCaller(),
	)
	if err != nil {
		return nil, err
	}

	return zl.Sugar(), nil
}

func SafeSync(logger *zap.SugaredLogger) {
	if logger == nil {
		return
	}
	if err := logger.Sync(); err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) && (errors.Is(pe.Err, syscall.EINVAL) || errors.Is(pe.Err, syscall.ENOTTY)) {
			return
		}
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY) {
			return
		}
		logger.Errorf("failed to sync logger: %w", err)
	}
}
