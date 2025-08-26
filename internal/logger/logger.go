package logger

import (
	"errors"
	"os"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.SugaredLogger
}

// Initialize singleton logger.
func Initialize(level string) (*Logger, error) {
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

	return &Logger{SugaredLogger: zl.Sugar()}, nil
}

func (l *Logger) SafeSync() {
	if l.SugaredLogger == nil {
		return
	}
	if err := l.SugaredLogger.Sync(); err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) && (errors.Is(pe.Err, syscall.EINVAL) || errors.Is(pe.Err, syscall.ENOTTY)) {
			return
		}
		if errors.Is(err, syscall.EINVAL) || errors.Is(err, syscall.ENOTTY) {
			return
		}
		l.SugaredLogger.Errorf("failed to sync logger: %w", err)
	}
}
