package fcm

import (
	"github.com/smancke/guble/server/metrics"
	"time"
)

var (
	mTotalSentMessages                = metrics.NewInt("fcm.total_sent_messages")
	mTotalSendErrors                  = metrics.NewInt("fcm.total_sent_message_errors")
	mTotalResponseErrors              = metrics.NewInt("fcm.total_response_errors")
	mTotalResponseInternalErrors      = metrics.NewInt("fcm.total_response_internal_errors")
	mTotalResponseNotRegisteredErrors = metrics.NewInt("fcm.total_response_not_registered_errors")
	mTotalReplacedCanonicalErrors     = metrics.NewInt("fcm.total_replaced_canonical_errors")
	mTotalResponseOtherErrors         = metrics.NewInt("fcm.total_response_other_errors")
	mMinute                           = metrics.NewMap("fcm.minute")
	mHour                             = metrics.NewMap("fcm.hour")
	mDay                              = metrics.NewMap("fcm.day")
)

const (
	currentTotalMessagesLatenciesKey = "current_messages_total_latencies_nanos"
	currentTotalMessagesKey          = "current_messages_count"
	currentTotalErrorsLatenciesKey   = "current_errors_total_latencies_nanos"
	currentTotalErrorsKey            = "current_errors_count"
	defaultAverageLatencyJSONValue   = "\"\""
	milliPerNano                     = 1000000
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
		msgLatenciesValue, msgNumberValue, milliPerNano, defaultAverageLatencyJSONValue)
	metrics.SetAverage(m, "last_errors_average_latency_msec",
		errLatenciesValue, errNumberValue, milliPerNano, defaultAverageLatencyJSONValue)
}

func resetIntervalMetrics(m metrics.Map, t time.Time) {
	m.Set("current_interval_start", metrics.NewTime(t))
	metrics.AddToMaps(currentTotalMessagesLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalMessagesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsLatenciesKey, 0, m)
	metrics.AddToMaps(currentTotalErrorsKey, 0, m)
}

func addToLatenciesAndCountsMaps(latenciesKey string, countKey string, latency time.Duration) {
	metrics.AddToMaps(latenciesKey, int64(latency), mMinute, mHour, mDay)
	metrics.AddToMaps(countKey, 1, mMinute, mHour, mDay)
}
