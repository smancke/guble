package protocol

import (
	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"runtime"
	"strings"
)

func ErrWithoutTrace(pattern string, args ...interface{}) {
	msg := fmt.Sprintf(pattern, args...)
	log.Printf("ERROR: %v", msg)
}

func PanicLogger() {
	if r := recover(); r != nil {
		log.Printf("PANIC (%v): %v", identifyLogOrigin(), r)
		log.Printf(getStackTraceMessage(fmt.Sprintf("%v", r)))
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
