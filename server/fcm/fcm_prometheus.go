package fcm

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_sent",
		Help: "Number of messages sent to FCM",
	})

	pSendErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_send_errors",
		Help: "Number of FCM errors when trying to send",
	})

	pResponseErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_response_errors",
		Help: "Number of FCM errors received as responses",
	})

	pResponseInternalErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_response_internal_errors",
		Help: "Number of internal errors related to FCM responses",
	})

	pResponseNotRegisteredErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_response_not_registered_errors",
		Help: "Number of errors related to not registered states in FCM connector",
	})

	pResponseReplacedCanonicalErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_response_replaced_canonical_errors",
		Help: "Number of errors related to canonical IDs in FCM connector",
	})

	pResponseOtherErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "fcm_response_other_errors",
		Help: "Number of other errors related to responses in the FCM connector",
	})
)

func init() {
	prometheus.MustRegister(
		pSent,
		pSendErrors,
		pResponseErrors,
		pResponseInternalErrors,
		pResponseNotRegisteredErrors,
		pResponseReplacedCanonicalErrors,
		pResponseOtherErrors,
	)
}
