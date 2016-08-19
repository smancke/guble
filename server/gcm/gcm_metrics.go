package gcm

import (
	"expvar"
	"github.com/smancke/guble/server/metrics"
	"strconv"
	"time"
)

var (
	mTotalSentMessages      = metrics.NewInt("guble.gcm.total_sent_messages")
	mTotalSentMessageErrors = metrics.NewInt("guble.gcm.total_sent_message_errors")

	mMinute = expvar.NewMap("guble.gcm.minute")
	mHour   = expvar.NewMap("guble.gcm.hour")
	mDay    = expvar.NewMap("guble.gcm.day")
)

const (
	currentTotalMessagesLatenciesKey = "current_total_message_latencies"
	currentTotalMessagesKey          = "current_total_messages"
	currentTotalErrorsLatenciesKey   = "current_total_errors_latencies"
	currentTotalErrorsKey            = "current_total_errors"
	averageMessageLatencyKey         = "average_message_latency"
	averageErrorLatencyKey           = "average_error_latency"
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
	setAverageLatency(m, averageMessageLatencyKey, msgLatenciesValue, msgNumberValue)
	setAverageLatency(m, averageErrorLatencyKey, errLatenciesValue, errNumberValue)
	//TODO Cosmin could add "t" as an expvar property (take care to escape it as JSON)
}

func setAverageLatency(m *expvar.Map, key string, totalValue, casesValue expvar.Var) {
	if totalValue != nil && casesValue != nil {
		total, _ := strconv.ParseInt(totalValue.String(), 10, 64)
		cases, _ := strconv.ParseInt(casesValue.String(), 10, 64)
		m.Set(key, newAverage(total, cases))
	} else {
		m.Set(key, newAverage(0, 0))
	}
}
