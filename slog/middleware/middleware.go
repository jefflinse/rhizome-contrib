// Package middleware provides a rhizome.Middleware that logs node
// execution events via log/slog.
package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/jefflinse/rhizome"
)

// Config configures the middleware. The zero value is valid and logs
// node start/end to slog.Default at Debug level.
type Config struct {
	// Logger is the slog.Logger to emit records on. Default: slog.Default.
	Logger *slog.Logger
	// StartLevel is the level used when a node begins executing.
	// Default: slog.LevelDebug.
	StartLevel slog.Level
	// EndLevel is the level used when a node returns successfully.
	// Default: slog.LevelDebug.
	EndLevel slog.Level
	// ErrorLevel is the level used when a node returns an error.
	// Default: slog.LevelError.
	ErrorLevel slog.Level
	// SuppressStart skips the pre-execution record when true.
	SuppressStart bool
}

// Option configures Config.
type Option func(*Config)

// WithLogger overrides the slog.Logger used by the middleware.
func WithLogger(l *slog.Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithStartLevel sets the level for the pre-execution record.
func WithStartLevel(level slog.Level) Option {
	return func(c *Config) { c.StartLevel = level }
}

// WithEndLevel sets the level for the post-execution success record.
func WithEndLevel(level slog.Level) Option {
	return func(c *Config) { c.EndLevel = level }
}

// WithErrorLevel sets the level for the post-execution error record.
func WithErrorLevel(level slog.Level) Option {
	return func(c *Config) { c.ErrorLevel = level }
}

// WithoutStartRecord suppresses the pre-execution record, emitting only the
// end record. Useful when you only care about completion timing.
func WithoutStartRecord() Option {
	return func(c *Config) { c.SuppressStart = true }
}

// New builds a rhizome.Middleware[S] that logs each node execution with
// slog. Attributes on every record:
//
//	rhizome.node      – node name
//	rhizome.duration  – execution time (end record only)
//	error             – error value (error record only)
func New[S any](opts ...Option) rhizome.Middleware[S] {
	cfg := Config{
		StartLevel: slog.LevelDebug,
		EndLevel:   slog.LevelDebug,
		ErrorLevel: slog.LevelError,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(ctx context.Context, node string, state S, next rhizome.NodeFunc[S]) (S, error) {
		logger := cfg.Logger
		if logger == nil {
			logger = slog.Default()
		}
		nodeAttr := slog.String("rhizome.node", node)

		if !cfg.SuppressStart {
			logger.LogAttrs(ctx, cfg.StartLevel, "rhizome: node start", nodeAttr)
		}

		start := time.Now()
		result, err := next(ctx, state)
		elapsed := time.Since(start)

		durAttr := slog.Duration("rhizome.duration", elapsed)
		if err != nil {
			logger.LogAttrs(ctx, cfg.ErrorLevel, "rhizome: node error",
				nodeAttr, durAttr, slog.Any("error", err))
		} else {
			logger.LogAttrs(ctx, cfg.EndLevel, "rhizome: node end",
				nodeAttr, durAttr)
		}

		return result, err
	}
}
