package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/alamo-ds/planner-elt/internal/plannerr"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/s-hammon/p"
	"gopkg.in/natefinch/lumberjack.v2"
)

type closeFunc func() error

func initLogger(logFile string) (*slog.Logger, closeFunc, error) {
	handlers := []slog.Handler{
		tint.NewHandler(os.Stderr, &tint.Options{
			Level:       slog.LevelDebug,
			ReplaceAttr: replaceAttr,
			// NoColor:     !(isatty.IsCygwinTerminal(os.Stderr.Fd()) || isatty.IsTerminal(os.Stderr.Fd())),
			NoColor: !isatty.IsCygwinTerminal(os.Stderr.Fd()) &&
				!isatty.IsTerminal(os.Stderr.Fd()),
		}),
	}
	closers := []closeFunc{}

	if logFile != "" {
		rotatingFile := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    500,
			MaxAge:     28,
			MaxBackups: 10,
			LocalTime:  false,
			Compress:   true,
		}

		handlers = append(handlers, slog.NewJSONHandler(rotatingFile, &slog.HandlerOptions{
			Level:       slog.LevelInfo,
			ReplaceAttr: replaceAttr,
		}))

		closers = append(closers, func() error {
			if err := rotatingFile.Close(); err != nil {
				return fmt.Errorf("failed to close log file: %w", err)
			}

			return nil
		})
	}

	closer := func() error {
		var errs []error
		for _, close := range closers {
			if err := close(); err != nil {
				errs = append(errs, err)
			}
		}

		return errors.Join(errs...)
	}

	return slog.New(slog.NewMultiHandler(handlers...)), closer, nil
}

type multiError interface {
	error
	Unwrap() []error
}

func errorAttrs(err error) []slog.Attr {
	attrs := []slog.Attr{
		{
			Key:   "message",
			Value: slog.StringValue(err.Error()),
		},
	}
	attrs = append(attrs, plannerr.Attrs(err)...)

	return attrs
}

func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == "error" {
		err, ok := a.Value.Any().(error)
		if !ok {
			return a
		}

		if multiErr, ok := errors.AsType[multiError](err); ok {
			var errAttrs []slog.Attr
			for i, e := range multiErr.Unwrap() {
				errAttrs = append(errAttrs, slog.GroupAttrs(p.Format("error_%d", i+1), errorAttrs(e)...))
			}

			return slog.GroupAttrs("errors", errAttrs...)
		}

		return slog.GroupAttrs("error", errorAttrs(err)...)
	}

	return a
}
