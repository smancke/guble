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

func Debug(pattern string, args ...interface{}) {
	if LogLevel >= LEVEL_DEBUG {
		log.Print("DEBUG: ", fmt.Sprintf(pattern, args...))
	}
}

func Info(pattern string, args ...interface{}) {
	if LogLevel >= LEVEL_INFO {
		log.Print("INFO: ", fmt.Sprintf(pattern, args...))
	}
}

func Err(pattern string, args ...interface{}) {
	if LogLevel >= LEVEL_ERR {
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
