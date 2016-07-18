package gubled

import (
	"bytes"
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLogstashFormatter_Format(t *testing.T) {

	assert := assert.New(t)

	lf := logstashFormatter{Type: "abc", ServiceName: "guble", Env: "prod"}

	fields := logrus.Fields{
		"message": "def",
		"level":   "ijk",
		"type":    "lmn",
		"one":     1,
		"pi":      3.14,
		"bool":    true,
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
	assert.Equal("application", data["log_type"])
	assert.Equal("service", data["application_type"])
	assert.Equal("guble", data["service"])
	assert.Equal("prod", data["environment"])

	assert.NotEmpty(data["@timestamp"])
	assert.Equal("abc", data["type"])
	assert.Equal("msg", data["message"])
	assert.Equal("info", data["loglevel"])

	// substituted fields
	assert.Equal("def", data["fields.message"])
	assert.Equal("lmn", data["fields.type"])

	// formats
	assert.Equal(json.Number("1"), data["one"])
	assert.Equal(json.Number("3.14"), data["pi"])
	assert.Equal(true, data["bool"])
}
