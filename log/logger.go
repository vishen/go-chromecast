package log

import (
	"io"

	"github.com/rs/zerolog"
)

type Fields map[string]interface{}

type Level int8

const (
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel Level = iota
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// FatalLevel level. Logs and then calls `os.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel
)

type Logger interface {
	Level() Level
	SetOutput(out io.Writer)
	SetLevel(level Level)
	WithField(key string, value interface{}) Logger
	WithFields(fields Fields) Logger
	WithError(err error) Logger

	Debug(msg string)
	Info(msg string)
	Print(msg string)
	Warn(msg string)
	Error(msg string)
	Fatal(msg string)
	Panic(msg string)

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})
}

type wrapLogger struct {
	zerolog.Logger
}

var _ Logger = &wrapLogger{}

func (wl *wrapLogger) Level() Level {
	return Level(wl.Logger.GetLevel())
}

func (wl *wrapLogger) SetOutput(out io.Writer) {
	wl.Logger = wl.Logger.Output(out)
}

func (wl *wrapLogger) SetLevel(level Level) {
	wl.Logger = wl.Logger.Level(zerolog.Level(level))
}

func (wl *wrapLogger) WithField(key string, value interface{}) Logger {
	wl.Logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Interface(key, value)
	})
	return wl
}

func (wl *wrapLogger) WithFields(fields Fields) Logger {
	wl.Logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Fields(fields)
	})
	return wl
}

func (wl *wrapLogger) WithError(err error) Logger {
	wl.Logger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		return c.Err(err)
	})
	return wl
}

func (wl *wrapLogger) Debugf(format string, args ...interface{}) {
	wl.Logger.Debug().Msgf(format, args...)
}

func (wl *wrapLogger) Infof(format string, args ...interface{}) {
	wl.Logger.Info().Msgf(format, args...)
}

func (wl *wrapLogger) Printf(format string, args ...interface{}) {
	wl.Logger.Printf(format, args...)
}

func (wl *wrapLogger) Warnf(format string, args ...interface{}) {
	wl.Logger.Warn().Msgf(format, args...)
}

func (wl *wrapLogger) Errorf(format string, args ...interface{}) {
	wl.Logger.Error().Msgf(format, args...)
}

func (wl *wrapLogger) Fatalf(format string, args ...interface{}) {
	wl.Logger.Fatal().Msgf(format, args...)
}

func (wl *wrapLogger) Panicf(format string, args ...interface{}) {
	wl.Logger.Panic().Msgf(format, args...)
}

func (wl *wrapLogger) Debug(msg string) {
	wl.Logger.Debug().Msg(msg)
}

func (wl *wrapLogger) Info(msg string) {
	wl.Logger.Info().Msg(msg)
}

func (wl *wrapLogger) Print(msg string) {
	wl.Logger.Printf(msg)
}

func (wl *wrapLogger) Warn(msg string) {
	wl.Logger.Warn().Msg(msg)
}

func (wl *wrapLogger) Error(msg string) {
	wl.Logger.Error().Msg(msg)
}

func (wl *wrapLogger) Fatal(msg string) {
	wl.Logger.Fatal().Msg(msg)
}

func (wl *wrapLogger) Panic(msg string) {
	wl.Logger.Panic().Msg(msg)
}

func New(out io.Writer) Logger {
	return &wrapLogger{zerolog.New(out).With().Timestamp().Logger().Level(zerolog.InfoLevel)}
}
