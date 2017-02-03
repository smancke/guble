package logformatter

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogstashFormatter_Format(t *testing.T) {
	a := assert.New(t)

	lf := LogstashFormatter{Type: "abc", ServiceName: "guble", Env: "prod"}

	fields := logrus.Fields{
		"msg":   "def",
		"level": "ijk",
		"type":  "lmn",
		"one":   1,
		"pi":    3.14,
		"bool":  true,
	}

	entry := logrus.WithFields(fields)
	entry.Message = "msg"
	entry.Level = logrus.InfoLevel

	b, _ := lf.Format(entry)

	var data map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	dec.Decode(&data)

	// base fields
	a.Equal("application", data["log_type"])
	a.Equal("service", data["application_type"])
	a.Equal("guble", data["service"])
	a.Equal("prod", data["environment"])

	a.NotEmpty(data["@timestamp"])
	a.NotEmpty(data["host"])
	a.Equal("abc", data["type"])
	a.Equal("msg", data["msg"])
	a.Equal("info", data["loglevel"])

	// substituted fields
	a.Equal("def", data["fields.msg"])
	a.Equal("lmn", data["fields.type"])

	// formats
	a.Equal(json.Number("1"), data["one"])
	a.Equal(json.Number("3.14"), data["pi"])
	a.Equal(true, data["bool"])
}
