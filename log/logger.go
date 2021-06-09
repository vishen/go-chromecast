package log

import (
	"context"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

type Fields map[string]interface{}

type Level uint32

var (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel = Level(logrus.PanicLevel)
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel = Level(logrus.FatalLevel)
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel = Level(logrus.ErrorLevel)
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel = Level(logrus.WarnLevel)
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel = Level(logrus.InfoLevel)
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel = Level(logrus.DebugLevel)
)

type LogEntry interface {
	Logger
	Fields() Fields
}

type Logger interface {
	SetLevel(level Level)
	GetLevel() Level
	SetOutput(out io.Writer)

	WithContext(ctx context.Context) LogEntry
	WithField(key string, value interface{}) LogEntry
	WithFields(fields Fields) LogEntry
	WithError(err error) LogEntry

	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Printf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Warningf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Panicf(format string, args ...interface{})

	Debug(args ...interface{})
	Info(args ...interface{})
	Print(args ...interface{})
	Warn(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
	Panic(args ...interface{})

	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Println(args ...interface{})
	Warnln(args ...interface{})
	Warningln(args ...interface{})
	Errorln(args ...interface{})
	Fatalln(args ...interface{})
	Panicln(args ...interface{})
}

var _ Logger = &wrapLogger{}

type wrapLogger struct {
	*logrus.Logger
}

func (dl *wrapLogger) GetLevel() Level {
	return Level(dl.Level)
}

func (dl *wrapLogger) SetLevel(level Level) {
	dl.Logger.SetLevel(logrus.Level(level))
}

func (dl *wrapLogger) WithContext(ctx context.Context) LogEntry {
	return &wrapLogEntry{dl.Logger.WithContext(ctx)}
}

func (dl *wrapLogger) WithField(key string, value interface{}) LogEntry {
	return &wrapLogEntry{dl.Logger.WithField(key, value)}
}

func (dl *wrapLogger) WithFields(fields Fields) LogEntry {
	return &wrapLogEntry{dl.Logger.WithFields(logrus.Fields(fields))}
}

func (dl *wrapLogger) WithError(err error) LogEntry {
	return &wrapLogEntry{dl.Logger.WithError(err)}
}

var _ LogEntry = &wrapLogEntry{}

type wrapLogEntry struct {
	*logrus.Entry
}

func (dle *wrapLogEntry) Fields() Fields {
	return Fields(dle.Entry.Data)
}

func (dle *wrapLogEntry) SetOutput(out io.Writer) {
	dle.Logger.SetOutput(out)
}

func (dle *wrapLogEntry) GetLevel() Level {
	return Level(dle.Entry.Level)
}

func (dle *wrapLogEntry) SetLevel(level Level) {
	dle.Logger.SetLevel(logrus.Level(level))
}

func (dle *wrapLogEntry) WithContext(ctx context.Context) LogEntry {
	return &wrapLogEntry{Entry: dle.Logger.WithContext(ctx)}
}

func (dle *wrapLogEntry) WithField(key string, value interface{}) LogEntry {
	return &wrapLogEntry{Entry: dle.Logger.WithField(key, value)}
}

func (dle *wrapLogEntry) WithFields(fields Fields) LogEntry {
	return &wrapLogEntry{Entry: dle.Logger.WithFields(logrus.Fields(fields))}
}

func (dle *wrapLogEntry) WithError(err error) LogEntry {
	return &wrapLogEntry{Entry: dle.Logger.WithError(err)}
}

func New(out io.Writer) Logger {
	l := &logrus.Logger{
		Out:          out,
		Formatter:    new(logrus.TextFormatter),
		Hooks:        make(logrus.LevelHooks),
		Level:        logrus.InfoLevel,
		ExitFunc:     os.Exit,
		ReportCaller: false,
	}
	return &wrapLogger{l}
}
