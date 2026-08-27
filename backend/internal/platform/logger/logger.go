package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger for application logging
type Logger struct {
	*zap.Logger
}

// New creates a new Logger instance
func New(level, format, output string) (*Logger, error) {
	// Parse log level
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.StacktraceKey = "stacktrace"
	encoderConfig.MessageKey = "message"
	encoderConfig.LevelKey = "level"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create encoder based on format
	var encoder zapcore.Encoder
	if format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create writer
	var writer zapcore.WriteSyncer
	if output == "stdout" {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		// Could add file output here
		writer = zapcore.AddSync(os.Stdout)
	}

	// Create core
	core := zapcore.NewCore(encoder, writer, zapLevel)

	// Create logger with caller info
	zapLogger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return &Logger{zapLogger}, nil
}

// NewDevelopment creates a logger with development settings
func NewDevelopment() (*Logger, error) {
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{zapLogger}, nil
}

// NewProduction creates a logger with production settings
func NewProduction() (*Logger, error) {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &Logger{zapLogger}, nil
}

// Helper methods for structured logging

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{l.Logger.With(zapFields...)}
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{l.Logger.With(zap.Any(key, value))}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{l.Logger.With(zap.Error(err))}
}

// HTTP request logging helpers

func (l *Logger) LogHTTPRequest(method, path string, statusCode int, duration float64) {
	l.Info("HTTP request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Float64("duration_ms", duration),
	)
}

func (l *Logger) LogHTTPError(method, path string, statusCode int, err error) {
	l.Error("HTTP error",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("status", statusCode),
		zap.Error(err),
	)
}


