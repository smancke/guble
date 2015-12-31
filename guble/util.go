package guble

import (
	"bytes"
	"errors"
)

// helper for collecting multiple errors
type ErrorList struct {
	errors            []error
	descriptionPrefix string
}

func NewErrorList(descriptionPrefix string) *ErrorList {
	return &ErrorList{
		descriptionPrefix: descriptionPrefix,
		errors:            []error{},
	}

}

// Add an error.
func (l *ErrorList) Add(err error) {
	l.errors = append(l.errors, err)
}

// returns an error containing the information of all errors in the list,
// of nil if the list is empthy
func (l *ErrorList) ErrorOrNil() error {
	if len(l.errors) == 0 {
		return nil
	}
	buffer := bytes.Buffer{}
	buffer.WriteString(l.descriptionPrefix)
	for i, err := range l.errors {
		buffer.WriteString(err.Error())
		if i+1 < len(l.errors) {
			buffer.WriteString("; ")
		}
	}
	return errors.New(buffer.String())
}
