package consumer

import "github.com/mylxsw/asteria/log"

type Logger struct{}

func (l Logger) Debug(args ...interface{}) {
}

func (l Logger) Info(args ...interface{}) {
}

func (l Logger) Warn(args ...interface{}) {
	log.Warningf("[queue] %v", args...)
}

func (l Logger) Error(args ...interface{}) {
	log.Errorf("[queue] %v", args...)
}

func (l Logger) Fatal(args ...interface{}) {
	log.Errorf("[queue] %v", args...)
}
