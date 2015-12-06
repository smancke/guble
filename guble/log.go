package guble

import (
	"bytes"
	"fmt"
	"log"
	"runtime"
	"strings"
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
		msg := fmt.Sprintf(pattern, args...)
		log.Printf("ERROR (%v): %v", identifyLogOrigin(), msg)
	}
}

func PanicLogger() {
	if r := recover(); r != nil {
		getStackTraceMessage(fmt.Sprintf("%v", r))
	}
}

func identifyLogOrigin() string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(3, pc[:])
	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		if !strings.HasPrefix(name, "runtime.") {
			break
		}
	}

	switch {
	case name != "":
		return fmt.Sprintf("%v:%v", name, line)
	case file != "":
		return fmt.Sprintf("%v:%v", file, line)
	}

	return fmt.Sprintf("pc:%x", pc)
}

func getStackTraceMessage(msg string) string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(3, pc[:])
	buff := &bytes.Buffer{}
	buff.WriteString(msg)
	buff.WriteString("\n")

	for _, pc := range pc[:n] {
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		file, line = fn.FileLine(pc)
		name = fn.Name()
		switch {
		case name != "":
			buff.WriteString(fmt.Sprintf("! %v:%v\n", name, line))
		case file != "":
			buff.WriteString(fmt.Sprintf("! %v:%v\n", file, line))
		}
	}
	return string(buff.Bytes())
}
