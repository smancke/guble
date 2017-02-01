package logformatter

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"os"
	"time"
)

const (
	defaultServiceName     = "guble"
	defaultLogType         = "application"
	defaultApplicationType = "service"
)

// LogstashFormatter generates json in logstash format.
// Logstash site: http://logstash.net/
type LogstashFormatter struct {

	//Type of the fields
	Type string // if not empty use for logstash type field.

	//Env is the environment on which the application is running
	Env string

	//ServiceName will be by default guble
	ServiceName string

	//ApplicationType will be  by default "service". Other values could be "service", "system", "appserver", "webserver"
	ApplicationType string

	//LogType will be by default  application. Other possible values "access", "error", "application", "system"
	LogType string

	//TimestampFormat sets the format used for timestamps.
	TimestampFormat string
}

// Format the logrus entry to a byte slice, or return an error.
func (f *LogstashFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	fields := make(logrus.Fields)

	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			// https://github.com/Sirupsen/logrus/issues/137
			// https://github.com/sirupsen/logrus/issues/377
			fields[k] = v.Error()
		default:
			fields[k] = v
		}
	}

	fields["environment"] = f.Env

	timeStampFormat := f.TimestampFormat
	if timeStampFormat == "" {
		timeStampFormat = time.RFC3339
	}

	fields["@timestamp"] = entry.Time.Format(timeStampFormat)

	if f.ServiceName != "" {
		fields["service"] = f.ServiceName
	} else {
		fields["service"] = defaultServiceName
	}

	if f.ApplicationType != "" {
		fields["application_type"] = f.ServiceName
	} else {
		fields["application_type"] = defaultApplicationType
	}

	if f.LogType != "" {
		fields["log_type"] = f.LogType
	} else {
		fields["log_type"] = defaultLogType
	}

	// set message field, prefixing fields clashes
	if v, ok := entry.Data["msg"]; ok {
		fields["fields.msg"] = v
	}
	fields["msg"] = entry.Message

	// set level field, prefixing fields clashes
	if v, ok := entry.Data["loglevel"]; ok {
		fields["fields.loglevel"] = v
	}
	fields["loglevel"] = entry.Level.String()

	//set host field, prefixing fields clashes
	if v, ok := entry.Data["host"]; ok {
		fields["fields.host"] = v
	}

	if hostname, err := os.Hostname(); err == nil {
		fields["host"] = hostname
	}

	// set type field
	if f.Type != "" {
		if v, ok := entry.Data["type"]; ok {
			fields["fields.type"] = v
		}
		fields["type"] = f.Type
	}

	serialized, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}
