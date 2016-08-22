package gcm

import (
	"github.com/smancke/guble/server/metrics"

	"expvar"
	"strconv"
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

func startGcmMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSentMessageErrors.Set(0)
	t := time.Now()
	resetCurrentMetrics(mMinute, t)
	resetCurrentMetrics(mHour, t)
	resetCurrentMetrics(mDay, t)
	go metrics.Every(time.Minute, processAndReset, mMinute)
	go metrics.Every(time.Hour, processAndReset, mHour)
	go metrics.Every(24*time.Hour, processAndReset, mDay)
}

func processAndReset(m metrics.Map, t time.Time) {
	msgLatenciesValue := m.Get(currentTotalMessagesLatenciesKey)
	msgNumberValue := m.Get(currentTotalMessagesKey)
	errLatenciesValue := m.Get(currentTotalErrorsLatenciesKey)
	errNumberValue := m.Get(currentTotalErrorsKey)

	resetCurrentMetrics(m, t)
	setCounter(m, "last_messages_count", msgNumberValue)
	setCounter(m, "last_errors_count", errNumberValue)
	setAverage(m, "last_messages_average_latency", msgLatenciesValue, msgNumberValue)
	setAverage(m, "last_errors_average_latency", errLatenciesValue, errNumberValue)
}

func setCounter(m metrics.Map, key string, value expvar.Var) {
	if value != nil {
		m.Set(key, value)
	} else {
		m.Set(key, metrics.ZeroValue)
	}
}

func setAverage(m metrics.Map, key string, totalVar, casesVar expvar.Var) {
	if totalVar != nil && casesVar != nil {
		total, _ := strconv.ParseInt(totalVar.String(), 10, 64)
		cases, _ := strconv.ParseInt(casesVar.String(), 10, 64)
		m.Set(key, metrics.NewAverage(total, cases, defaultAverageLatencyJSONValue))
	} else {
		m.Set(key, metrics.ZeroValue)
	}
}

func resetCurrentMetrics(m metrics.Map, t time.Time) {
	m.Set("current_interval_start", metrics.NewTimeVar(t))
	addToMetrics(currentTotalMessagesLatenciesKey, 0, m)
	addToMetrics(currentTotalMessagesKey, 0, m)
	addToMetrics(currentTotalErrorsLatenciesKey, 0, m)
	addToMetrics(currentTotalErrorsKey, 0, m)
}

func addToMetrics(key string, value int64, maps ...metrics.Map) {
	for _, m := range maps {
		m.Add(key, value)
	}
}
