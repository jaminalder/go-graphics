// Package logging records bounded operational events without visitor credentials.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New creates a service-tagged text logger; an empty level means info.
func New(service, level string, output io.Writer) (*slog.Logger, error) {
	severity := slog.LevelInfo
	if level != "" {
		if err := severity.UnmarshalText([]byte(strings.ToUpper(level))); err != nil {
			return nil, fmt.Errorf("invalid ART_LOG_LEVEL: %w", err)
		}
	}
	return slog.New(slog.NewTextHandler(output, &slog.HandlerOptions{Level: severity})).With("service", service), nil
}
