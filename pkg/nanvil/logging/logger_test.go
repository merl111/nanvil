package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestNewTextLogger(t *testing.T) {
	log, err := New("text", "info")
	require.NoError(t, err)
	require.NotNil(t, log)
	require.Equal(t, zapcore.InfoLevel, log.Level())
}

func TestNewJSONLogger(t *testing.T) {
	log, err := New("json", "warn")
	require.NoError(t, err)
	require.NotNil(t, log)
	require.Equal(t, zapcore.WarnLevel, log.Level())
}

func TestNewFormatAliases(t *testing.T) {
	for _, format := range []string{"", "console", "plain", "TEXT"} {
		log, err := New(format, "info")
		require.NoError(t, err, format)
		require.NotNil(t, log)
	}
}

func TestNewInvalidFormat(t *testing.T) {
	_, err := New("xml", "info")
	require.Error(t, err)
}

func TestNewInvalidLevel(t *testing.T) {
	_, err := New("text", "verbose")
	require.Error(t, err)
}

func TestIsText(t *testing.T) {
	require.True(t, IsText(""))
	require.True(t, IsText("text"))
	require.True(t, IsText("console"))
	require.False(t, IsText("json"))
}
