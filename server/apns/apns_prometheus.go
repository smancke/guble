package apns

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pSentMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_sent_messages",
		Help: "Number of messages sent to APNS",
	})
)

func init() {
	prometheus.MustRegister(
		pSentMessages,
	)
}
