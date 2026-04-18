package logging

import (
	"log/slog"
	"os"
)

// Level constants matching the levels used across the codebase.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

var level = new(slog.LevelVar) // default: Info

// SetLevel configures the minimum log level ("debug" or "info").
func SetLevel(logLevel string) error {
	switch logLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	default:
		level.Set(slog.LevelInfo)
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	slog.SetDefault(slog.New(h))
	return nil
}

func Info(message string, args ...any) {
	slog.Info(message, args...)
}

func Debug(message string, args ...any) {
	slog.Debug(message, args...)
}

func Warn(message string, args ...any) {
	slog.Warn(message, args...)
}

func Error(message string, args ...any) {
	slog.Error(message, args...)
}

// Fatal logs at Error level and exits with code 1.
func Fatal(message string, args ...any) {
	slog.Error(message, args...)
	os.Exit(1)
}
