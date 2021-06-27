package log

import (
	"io"

	"github.com/rs/zerolog"
)

// Formatter transforms the input into a formatted string.
type Formatter func(interface{}) string

type ConsoleWriterOptions struct {
	// NoColor disables the colorized output.
	NoColor bool

	// TimeFormat specifies the format for timestamp in output.
	TimeFormat string

	FormatTimestamp     Formatter
	FormatLevel         Formatter
	FormatCaller        Formatter
	FormatMessage       Formatter
	FormatFieldName     Formatter
	FormatFieldValue    Formatter
	FormatErrFieldName  Formatter
	FormatErrFieldValue Formatter
}

func NewConsoleWriter(out io.Writer, options ...func(*ConsoleWriterOptions)) io.Writer {
	opts := &ConsoleWriterOptions{}
	for _, fn := range options {
		fn(opts)
	}
	return zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
		w.Out = out
		w.NoColor = opts.NoColor
		if opts.TimeFormat != "" {
			w.TimeFormat = opts.TimeFormat
		}
		if opts.FormatTimestamp != nil {
			w.FormatTimestamp = zerolog.Formatter(opts.FormatTimestamp)
		}
		if opts.FormatLevel != nil {
			w.FormatLevel = zerolog.Formatter(opts.FormatLevel)
		}
		if opts.FormatCaller != nil {
			w.FormatCaller = zerolog.Formatter(opts.FormatCaller)
		}
		if opts.FormatMessage != nil {
			w.FormatMessage = zerolog.Formatter(opts.FormatMessage)
		}
		if opts.FormatFieldName != nil {
			w.FormatFieldName = zerolog.Formatter(opts.FormatFieldName)
		}
		if opts.FormatFieldValue != nil {
			w.FormatFieldValue = zerolog.Formatter(opts.FormatFieldValue)
		}
		if opts.FormatErrFieldName != nil {
			w.FormatErrFieldName = zerolog.Formatter(opts.FormatErrFieldName)
		}
		if opts.FormatErrFieldValue != nil {
			w.FormatErrFieldValue = zerolog.Formatter(opts.FormatErrFieldValue)
		}

	})
}
