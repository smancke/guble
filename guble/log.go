package guble

import (
	"fmt"
	"log"
)

func Debug(pattern string, args ...interface{}) {
	log.Println("DEBUG: " + fmt.Sprintf(pattern, args...))
}

func Err(pattern string, args ...interface{}) {
	log.Println("ERROR: " + fmt.Sprintf(pattern, args...))
}

func Info(pattern string, args ...interface{}) {
	log.Println("INFO: " + fmt.Sprintf(pattern, args...))
}
