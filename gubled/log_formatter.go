package gubled

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"time"
)

// LogstashGubleFormatter generates json in logstash format.
// Logstash site: http://logstash.net/
type logstashFormatter struct {

	//Type of the fields
	Type string // if not empty use for logstash type field.

	//Env is the environment on which the application is running
	Env string

	//ServiceName will be by default guble
	ServiceName string

	//ApplicationType will be  by default "service".Other values could be "service", "system", "appserver", "webserver"
	ApplicationType string

	//LogType will be by default  application.Other possible values "access", "error", "application", "system"
	LogType string

	// TimestampFormat sets the format used for timestamps.
	TimestampFormat string
}

func (f *logstashFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	fields := make(logrus.Fields)
	for k, v := range entry.Data {
		fields[k] = v
	}

	timeStampFormat := f.TimestampFormat
	if timeStampFormat == "" {
		timeStampFormat = time.RFC3339
	}

	fields["@timestamp"] = entry.Time.Format(timeStampFormat)

	if f.ServiceName != "" {
		fields["service"] = f.ServiceName
	} else {
		fields["service"] = "guble"
	}

	if f.ApplicationType != "" {
		fields["application_type"] = f.ServiceName
	} else {
		fields["application_type"] = "service"
	}

	if f.LogType != "" {
		fields["log_type"] = f.LogType
	} else {
		fields["log_type"] = "application"
	}

	fields["environment"] = f.Env

	// set message field
	v, ok := entry.Data["message"]
	if ok {
		fields["fields.message"] = v
	}
	fields["message"] = entry.Message

	// set level field
	v, ok = entry.Data["level"]
	if ok {
		fields["fields.level"] = v
	}
	fields["loglevel"] = entry.Level.String()

	// set type field
	if f.Type != "" {
		v, ok = entry.Data["type"]
		if ok {
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
