package main

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

type logHandler struct {
	verbose bool
	attrs   []*slog.Attr
}

// Enabled implements slog.Handler.
func (h *logHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if h.verbose {
		return true
	}
	return level >= 0
}

// Handle implements slog.Handler.
func (h *logHandler) Handle(ctx context.Context, r slog.Record) error {
	var fn = color.MagentaString
	switch r.Level {
	case slog.LevelInfo:
		fn = color.GreenString
	case slog.LevelWarn:
		fn = color.YellowString
	case slog.LevelError:
		fn = color.RedString
	}
	var sb strings.Builder
	sb.WriteString(color.HiBlackString(r.Time.Format("15:04:05.000")))
	sb.WriteByte(' ')
	sb.WriteString(fn(r.Message))
	for _, attr := range h.attrs {
		appendAttr(&sb, attr)
	}
	r.Attrs(func(attr slog.Attr) bool {
		appendAttr(&sb, &attr)
		if attr.Key == "err" {
			frames := runtime.CallersFrames([]uintptr{r.PC})
			for {
				frame, more := frames.Next()
				fmt.Fprintf(&sb, "\n  %s at %s:%d", frame.Function, color.HiBlackString(frame.File), frame.Line)
				if !more {
					break
				}
			}
		}
		return true
	})
	println(sb.String())
	return nil
}

var clrBold = color.New(color.Bold, color.FgHiBlack)

func appendAttr(sb *strings.Builder, attr *slog.Attr) {
	sb.WriteByte(' ')
	clrBold.Fprint(sb, attr.Key)
	sb.WriteByte('=')
	if attr.Key == "err" && attr.Value.Kind() == slog.KindAny {
		val := attr.Value.Any()
		if err, ok := val.(error); ok {
			sb.WriteString(err.Error())
			return
		}
	}
	sb.WriteString(attr.Value.String())
}

// WithAttrs implements slog.Handler.
func (h *logHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := h.attrs
	for _, attr := range attrs {
		newAttrs = append(newAttrs, &attr)
	}
	clone := &logHandler{
		h.verbose, newAttrs,
	}
	return clone
}

// WithGroup implements slog.Handler.
func (h *logHandler) WithGroup(name string) slog.Handler {
	return h
}

var _ slog.Handler = &logHandler{}
