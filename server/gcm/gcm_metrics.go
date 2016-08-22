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
	currentTotalMessagesLatenciesKey = "current_total_message_latencies"
	currentTotalMessagesKey          = "current_total_messages"
	currentTotalErrorsLatenciesKey   = "current_total_errors_latencies"
	currentTotalErrorsKey            = "current_total_errors"
	defaultAverageLatencyJSONValue   = "0"
)

func startGcmMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSentMessageErrors.Set(0)
	go every(time.Minute, processAndResetMap, mMinute)
	go every(time.Hour, processAndResetMap, mHour)
	go every(24*time.Hour, processAndResetMap, mDay)
}

func every(d time.Duration, f func(time.Time, *expvar.Map), m *expvar.Map) {
	for x := range time.Tick(d) {
		f(x, m)
	}
}

func processAndResetMap(t time.Time, m *expvar.Map) {
	logger.Debug("process and reset metrics map")
	msgLatenciesValue := m.Get(currentTotalMessagesLatenciesKey)
	msgNumberValue := m.Get(currentTotalMessagesKey)
	errLatenciesValue := m.Get(currentTotalErrorsLatenciesKey)
	errNumberValue := m.Get(currentTotalErrorsKey)
	m.Init()
	m.Set("current_interval_start", metrics.NewTimeVar(t))
	setAverageLatency(m, "average_message_latency", msgLatenciesValue, msgNumberValue)
	setAverageLatency(m, "average_error_latency", errLatenciesValue, errNumberValue)
}

func setAverageLatency(m *expvar.Map, key string, totalValue, casesValue expvar.Var) {
	if totalValue != nil && casesValue != nil {
		total, _ := strconv.ParseInt(totalValue.String(), 10, 64)
		cases, _ := strconv.ParseInt(casesValue.String(), 10, 64)
		m.Set(key, metrics.NewAverage(total, cases, defaultAverageLatencyJSONValue))
	} else {
		m.Set(key, metrics.NewAverage(0, 0, defaultAverageLatencyJSONValue))
	}
}

func updateMetricsMaps(valueKey string, counterKey string, value int64) {
	mMinute.Add(valueKey, value)
	mMinute.Add(counterKey, 1)
	mHour.Add(valueKey, value)
	mHour.Add(counterKey, 1)
	mDay.Add(valueKey, value)
	mDay.Add(counterKey, 1)
}
