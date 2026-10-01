package log

import (
	loglib "github.com/ygrebnov/log"
)

type Level = loglib.Level
type Record = loglib.Record
type Field = loglib.Field

const (
	LevelDebug = loglib.LevelDebug
	LevelError = loglib.LevelError
	LevelInfo  = loglib.LevelInfo
	LevelWarn  = loglib.LevelWarn
)

var (
	String = loglib.String
	Int    = loglib.Int
	Bool   = loglib.Bool
	Err    = loglib.Err

	NewRecord = loglib.NewRecord
)

type Logger interface {
	Log(level Level, message string, fields ...Field)
	LogRecord(record Record)
	Debug(message string, fields ...Field)
	Info(message string, fields ...Field)
	Warn(message string, fields ...Field)
	Error(message string, fields ...Field)
	With(fields ...Field) Logger
	Close() error
}

func NewSilentLogger() (Logger, error) {
	l, err := loglib.NewSilentLogger()
	if err != nil {
		return nil, err
	}

	return &logger{Logger: l}, nil
}

func NewLogger(cfg *loglib.Config) (Logger, error) {
	l, err := loglib.NewLogger(cfg)
	if err != nil {
		return nil, err
	}

	return &logger{Logger: l}, nil
}

type logger struct {
	*loglib.Logger
}

func (l *logger) With(fields ...Field) Logger {
	return &logger{
		Logger: l.Logger.With(fields...),
	}
}
