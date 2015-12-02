package guble

import (
	"fmt"
	"log"
	"runtime"
)

const (
	LEVEL_DEBUG = iota
	LEVEL_INFO
	LEVEL_ERR
)

var LogLevel = LEVEL_INFO

func DebugEnabled() bool {
	return LogLevel <= LEVEL_DEBUG
}

func Debug(pattern string, args ...interface{}) {
	if DebugEnabled() {
		log.Print("DEBUG: ", fmt.Sprintf(pattern, args...))
	}
}

func InfoEnabled() bool {
	return LogLevel <= LEVEL_INFO
}

func Info(pattern string, args ...interface{}) {
	if InfoEnabled() {
		log.Print("INFO: ", fmt.Sprintf(pattern, args...))
	}
}

func ErrEnabled() bool {
	return LogLevel <= LEVEL_ERR
}

func Err(pattern string, args ...interface{}) {
	if ErrEnabled() {
		pc := make([]uintptr, 1)
		runtime.Callers(2, pc)
		f := runtime.FuncForPC(pc[0])

		msg := fmt.Sprintf(pattern, args...)
		log.Printf("ERROR (%v):%v", f.Name(), msg)
	}
}

func PanicLogger() {
	if r := recover(); r != nil {
		pc := make([]uintptr, 1)
		runtime.Callers(2, pc)
		f := runtime.FuncForPC(pc[0])

		log.Printf("PANIC (%v): %v", f.Name(), r)
	}
}
