package gcm

import (
	"expvar"
	"fmt"
	"github.com/smancke/guble/server/metrics"
	"strconv"
	"time"
)

var (
	mTotalSentMessages      = metrics.NewInt("guble.gcm.total_sent_messages")
	mTotalSentMessageErrors = metrics.NewInt("guble.gcm.total_sent_message_errors")

	mapMinute = expvar.NewMap("guble.gcm.minute")
	mapHour   = expvar.NewMap("guble.gcm.hour")
	mapDay    = expvar.NewMap("guble.gcm.day")
)

const (
	currentTotalLatenciesKey = "currentTotalLatencies"
	currentTotalMessagesKey  = "currentTotalMessages"
)

func startGcmMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSentMessageErrors.Set(0)
	mapMinute.Init()
	mapHour.Init()
	mapDay.Init()
	go every(time.Second*5, processAndResetMap, mapMinute)
	go every(time.Hour, processAndResetMap, mapHour)
	go every(24*time.Hour, processAndResetMap, mapDay)
}

func every(d time.Duration, f func(time.Time, *expvar.Map), m *expvar.Map) {
	for x := range time.Tick(d) {
		f(x, m)
	}
}

type Average struct {
	Total int64
	Cases int64
}

func newAverage(total int64, cases int64) Average {
	return Average{
		Total: total,
		Cases: cases,
	}
}

func (a Average) String() string {
	return fmt.Sprintf("%v", a.Total/a.Cases)
}

func processAndResetMap(t time.Time, m *expvar.Map) {
	logger.Debug("process and reset metrics map")

	latenciesValue := m.Get(currentTotalLatenciesKey)
	messagesValue := m.Get(currentTotalMessagesKey)

	m.Init()
	if latenciesValue != nil && messagesValue != nil {
		latencies, _ := strconv.ParseInt(latenciesValue.String(), 10, 64)
		messages, _ := strconv.ParseInt(messagesValue.String(), 10, 64)
		if messages > 0 {
			m.Set("averageLatency", newAverage(latencies, messages))
		}
	}
}
