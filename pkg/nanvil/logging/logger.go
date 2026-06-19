package logging

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// FormatText is human-readable console output (Anvil-style default).
	FormatText = "text"
	// FormatJSON is structured JSON, one object per line.
	FormatJSON = "json"
)

// New builds a zap logger for nanvil.
// format: "text" (default), "json", or aliases "console"/"plain".
// level: "debug", "info", "warn", "error" (default "info").
func New(format, level string) (*zap.Logger, error) {
	format = normalizeFormat(format)
	if format == "" {
		format = FormatText
	}
	lvl, err := parseLevel(level)
	if err != nil {
		return nil, err
	}
	switch format {
	case FormatText:
		return newTextLogger(lvl)
	case FormatJSON:
		return newJSONLogger(lvl)
	default:
		return nil, fmt.Errorf("unknown log format %q (use text or json)", format)
	}
}

// IsText reports whether format selects human-readable logging.
func IsText(format string) bool {
	return normalizeFormat(format) != FormatJSON
}

func normalizeFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text", "console", "plain":
		return FormatText
	case "json":
		return FormatJSON
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

func parseLevel(level string) (zapcore.Level, error) {
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" {
		level = "info"
	}
	var lvl zapcore.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return 0, fmt.Errorf("unknown log level %q", level)
	}
	return lvl, nil
}

func newTextLogger(lvl zapcore.Level) (*zap.Logger, error) {
	cfg := zap.NewDevelopmentConfig()
	cfg.Encoding = "console"
	cfg.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.DisableStacktrace = true
	cfg.DisableCaller = true
	return cfg.Build()
}

func newJSONLogger(lvl zapcore.Level) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	return cfg.Build()
}
