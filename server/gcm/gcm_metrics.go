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
	mMinute                 = expvar.NewMap("guble.gcm.minute")
	mHour                   = expvar.NewMap("guble.gcm.hour")
	mDay                    = expvar.NewMap("guble.gcm.day")
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
	resetCurrentMetrics()
	go metrics.Every(time.Minute, processAndReset, mMinute)
	go metrics.Every(time.Hour, processAndReset, mHour)
	go metrics.Every(24*time.Hour, processAndReset, mDay)
}

func processAndReset(m *expvar.Map, t time.Time) {
	logger.Debug("process and reset metrics map")
	msgLatenciesValue := m.Get(currentTotalMessagesLatenciesKey)
	msgNumberValue := m.Get(currentTotalMessagesKey)
	errLatenciesValue := m.Get(currentTotalErrorsLatenciesKey)
	errNumberValue := m.Get(currentTotalErrorsKey)

	m.Init()
	resetCurrentMetrics()
	m.Set("current_interval_start", metrics.NewTimeVar(t))
	setCounter(m, "last_messages_count", msgNumberValue)
	setCounter(m, "last_errors_count", errNumberValue)
	setAverage(m, "last_messages_average_latency", msgLatenciesValue, msgNumberValue)
	setAverage(m, "last_errors_average_latency", errLatenciesValue, errNumberValue)
}

func setCounter(m *expvar.Map, key string, value expvar.Var) {
	if value != nil {
		m.Set(key, value)
	} else {
		m.Set(key, metrics.ZeroValue)
	}
}

func setAverage(m *expvar.Map, key string, totalValue, casesValue expvar.Var) {
	if totalValue != nil && casesValue != nil {
		total, _ := strconv.ParseInt(totalValue.String(), 10, 64)
		cases, _ := strconv.ParseInt(casesValue.String(), 10, 64)
		m.Set(key, metrics.NewAverage(total, cases, defaultAverageLatencyJSONValue))
	} else {
		m.Set(key, metrics.ZeroValue)
	}
}

func resetCurrentMetrics() {
	addToMetrics(currentTotalMessagesLatenciesKey, 0)
	addToMetrics(currentTotalMessagesKey, 0)
	addToMetrics(currentTotalErrorsLatenciesKey, 0)
	addToMetrics(currentTotalErrorsKey, 0)
}

func addToMetrics(key string, value int64) {
	mMinute.Add(key, value)
	mHour.Add(key, value)
	mDay.Add(key, value)
}
