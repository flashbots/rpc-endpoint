package server

import (
	"fmt"
	"log"
)

type Logger interface {
	log(format string, v ...interface{})
	logError(format string, v ...interface{})
	CreateChildLogger(suffix string) Logger
}

type Log struct {
	uid string
}

func NewLogger(uid string) Logger {
	return &Log{uid: uid}
}
func (l *Log) log(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ", l.uid)
	log.Printf(prefix+format, v...)
}

func (l *Log) logError(format string, v ...interface{}) {
	prefix := fmt.Sprintf("[%s] ERROR: ", l.uid)
	log.Printf(prefix+format, v...)
}

func (l *Log) CreateChildLogger(suffix string) Logger {
	return NewLogger(l.uid + "/" + suffix)
}
