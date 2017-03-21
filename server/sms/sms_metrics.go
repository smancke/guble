package sms

import (
	"time"

	"github.com/cosminrentea/gobbler/server/metrics"
)

var (
	ns                           = metrics.NS("sms")
	mTotalSentMessages           = ns.NewInt("total_sent_messages")
	mTotalSendErrors             = ns.NewInt("total_sent_message_errors")
	mTotalResponseErrors         = ns.NewInt("total_response_errors")
	mTotalResponseInternalErrors = ns.NewInt("total_response_internal_errors")
	mTotalExpiredMessages        = ns.NewInt("total_expires_messages")
	mMinute                      = ns.NewMap("minute")
	mHour                        = ns.NewMap("hour")
	mDay                         = ns.NewMap("day")
)

const (
	currentTotalMessagesLatenciesKey = "current_messages_total_latencies_nanos"
	currentTotalMessagesKey          = "current_messages_count"
	currentTotalErrorsLatenciesKey   = "current_errors_total_latencies_nanos"
	currentTotalErrorsKey            = "current_errors_count"
)

func processAndResetIntervalMetrics(m metrics.Map, td time.Duration, t time.Time) {
	msgLatenciesValue := m.Get(currentTotalMessagesLatenciesKey)
	msgNumberValue := m.Get(currentTotalMessagesKey)
	errLatenciesValue := m.Get(currentTotalErrorsLatenciesKey)
	errNumberValue := m.Get(currentTotalErrorsKey)

	m.Init()
	resetIntervalMetrics(m, t)
	metrics.SetRate(m, "last_messages_rate_sec", msgNumberValue, td, time.Second)
	metrics.SetRate(m, "last_errors_rate_sec", errNumberValue, td, time.Second)
	metrics.SetAverage(m, "last_messages_average_latency_msec",
		msgLatenciesValue, msgNumberValue, metrics.MilliPerNano, metrics.DefaultAverageLatencyJSONValue)
	metrics.SetAverage(m, "last_errors_average_latency_msec",
		errLatenciesValue, errNumberValue, metrics.MilliPerNano, metrics.DefaultAverageLatencyJSONValue)
}

func resetIntervalMetrics(m metrics.Map, t time.Time) {
	m.Set("current_interval_start", metrics.NewTime(t))
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalMessagesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsKey, 0, m)
}
