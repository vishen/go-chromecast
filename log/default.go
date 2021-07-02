package log

import (
	"io"
	"os"
)

var std = New(os.Stdout)

func SetLogger(logger Logger) {
	std = logger
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	std.SetOutput(out)
}

// SetLevel sets the standard logger level.
func SetLevel(level Level) {
	std.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() Level {
	return std.Level()
}

// WithError creates an entry from the standard logger and adds an error to it, using the value defined in ErrorKey as key.
func WithError(err error) Logger {
	return std.WithError(err)
}

// WithField creates an entry from the standard logger and adds a field to
// it. If you want multiple fields, use `WithFields`.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithField(key string, value interface{}) Logger {
	return std.WithField(key, value)
}

// WithFields creates an entry from the standard logger and adds multiple
// fields to it. This is simply a helper for `WithField`, invoking it
// once for each field.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithFields(fields Fields) Logger {
	return std.WithFields(fields)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(msg string) {
	std.Debug(msg)
}

// Print logs a message at level Info on the standard logger.
func Print(msg string) {
	std.Print(msg)
}

// Info logs a message at level Info on the standard logger.
func Info(msg string) {
	std.Info(msg)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(msg string) {
	std.Warn(msg)
}

// Error logs a message at level Error on the standard logger.
func Error(msg string) {
	std.Error(msg)
}

// Panic logs a message at level Panic on the standard logger.
func Panic(msg string) {
	std.Panic(msg)
}

// Fatal logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
func Fatal(msg string) {
	std.Fatal(msg)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	std.Debugf(format, args...)
}

// Printf logs a message at level Info on the standard logger.
func Printf(format string, args ...interface{}) {
	std.Printf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	std.Infof(format, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	std.Warnf(format, args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	std.Errorf(format, args...)
}

// Panicf logs a message at level Panic on the standard logger.
func Panicf(format string, args ...interface{}) {
	std.Panicf(format, args...)
}

// Fatalf logs a message at level Fatal on the standard logger then the process will exit with status set to 1.
func Fatalf(format string, args ...interface{}) {
	std.Fatalf(format, args...)
}
