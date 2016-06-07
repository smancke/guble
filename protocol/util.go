package protocol

import (
	"bytes"
	"errors"
)

// ErrorList is a helper struct for collecting multiple errors.
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

// Add adds an error.
func (l *ErrorList) Add(err error) {
	l.errors = append(l.errors, err)
}

// ErrorOrNil returns an error containing the information of all errors in the list,
// of nil if the list is empty.
func (l *ErrorList) ErrorOrNil() error {
	if len(l.errors) == 0 {
		return nil
	}
	return errors.New(l.Error())
}

func (l *ErrorList) Error() string {
	if len(l.errors) == 0 {
		return ""
	}
	buffer := bytes.Buffer{}
	buffer.WriteString(l.descriptionPrefix)
	for i, err := range l.errors {
		buffer.WriteString(err.Error())
		if i+1 < len(l.errors) {
			buffer.WriteString("; ")
		}
	}
	return buffer.String()
}
