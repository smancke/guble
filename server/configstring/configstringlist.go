package configstring

import (
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

type List []string

func NewFromKingpin(settings kingpin.Settings) *List {
	sl := make(List, 0)
	settings.SetValue(&sl)
	return &sl
}

func (sl *List) Set(value string) error {
	delimiter := " "
	if strings.Contains(value, ",") {
		delimiter = ","
	}
	slice := strings.Split(value, delimiter)
	for _, s := range slice {
		if s != "" {
			*sl = append(*sl, s)
		}
	}
	return nil
}

func (sl *List) IsEmpty() bool {
	return len(*sl) == 0
}

func (sl List) String() string {
	res := "["
	for i, s := range sl {
		if i == 0 {
			res = res + s
		} else {
			res = res + " " + s
		}
	}
	res = res + "]"
	return res
}
