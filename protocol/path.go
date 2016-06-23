package protocol

import "strings"

// Path is the path of a topic
type Path string

func (path Path) Partition() string {
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return strings.SplitN(string(path), "/", 2)[0]
}
