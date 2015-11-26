package guble

import (
	"fmt"
	"log"
	"runtime"
)

func Debug(pattern string, args ...interface{}) {
	log.Println("DEBUG: " + fmt.Sprintf(pattern, args...))
}

func Err(pattern string, args ...interface{}) {
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc)
	f := runtime.FuncForPC(pc[0])

	msg := fmt.Sprintf(pattern, args...)
	log.Printf("ERROR (%v):%v", f.Name(), msg)
}

func Info(pattern string, args ...interface{}) {
	log.Println("INFO: " + fmt.Sprintf(pattern, args...))
}

func PanicLogger() {
	if r := recover(); r != nil {
		pc := make([]uintptr, 1)
		runtime.Callers(2, pc)
		f := runtime.FuncForPC(pc[0])

		log.Printf("PANIC (%v): %v", f.Name(), r)
	}
}
