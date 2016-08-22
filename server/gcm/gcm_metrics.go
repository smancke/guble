package gcm

import (
	"github.com/smancke/guble/server/metrics"
	"time"
)

var (
	mTotalSentMessages      = metrics.NewInt("guble.gcm.total_sent_messages")
	mTotalSentMessageErrors = metrics.NewInt("guble.gcm.total_sent_message_errors")
	mMinute                 = metrics.NewMap("guble.gcm.minute")
	mHour                   = metrics.NewMap("guble.gcm.hour")
	mDay                    = metrics.NewMap("guble.gcm.day")
)

const (
	currentTotalMessagesLatenciesKey = "current_messages_total_latencies"
	currentTotalMessagesKey          = "current_messages_count"
	currentTotalErrorsLatenciesKey   = "current_errors_total_latencies"
	currentTotalErrorsKey            = "current_errors_count"
	defaultAverageLatencyJSONValue   = "0"
)

func startGCMMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSentMessageErrors.Set(0)
	t := time.Now()
	resetCurrentMetrics(mMinute, t)
	resetCurrentMetrics(mHour, t)
	resetCurrentMetrics(mDay, t)
	go metrics.Every(time.Minute, processAndReset, mMinute)
	go metrics.Every(time.Hour, processAndReset, mHour)
	go metrics.Every(time.Hour*24, processAndReset, mDay)
}

func processAndReset(m metrics.Map, t time.Time) {
	msgLatenciesValue := m.Get(currentTotalMessagesLatenciesKey)
	msgNumberValue := m.Get(currentTotalMessagesKey)
	errLatenciesValue := m.Get(currentTotalErrorsLatenciesKey)
	errNumberValue := m.Get(currentTotalErrorsKey)

	m.Init()
	resetCurrentMetrics(m, t)
	metrics.SetCounter(m, "last_messages_count", msgNumberValue)
	metrics.SetCounter(m, "last_errors_count", errNumberValue)
	metrics.SetAverage(m, "last_messages_average_latency", msgLatenciesValue, msgNumberValue, defaultAverageLatencyJSONValue)
	metrics.SetAverage(m, "last_errors_average_latency", errLatenciesValue, errNumberValue, defaultAverageLatencyJSONValue)
}

func resetCurrentMetrics(m metrics.Map, t time.Time) {
	m.Set("current_interval_start", metrics.NewTimeVar(t))
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalMessagesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsKey, 0, m)
}
