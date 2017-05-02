package apns

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pSentMessages = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_sent_messages",
		Help: "Number of messages sent to APNS",
	})

	pSendErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_send_errors",
		Help: "Number of errors when trying to send messages to APNS",
	})

	pResponseErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_response_errors",
		Help: "Number of errors received after sending messages to APNS",
	})

	pResponseInternalErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_response_internal_errors",
		Help: "Number of internal errors related to handling responses from APNS",
	})

	pResponseRegistrationErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_response_registration_errors",
		Help: "Number of errors related to APNS registrations",
	})

	pResponseOtherErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_response_other_errors",
		Help: "Number of other APNS errors",
	})

	pSendNetworkErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_send_network_errors",
		Help: "Number of errors related to network when sending to APNS",
	})

	pSendRetryCloseTLS = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_send_retry_close_tls",
		Help: "Number of retries related to closing TLS in the APNS connector",
	})

	pSendRetryUnrecoverable = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "apns_send_retry_unrecoverable",
		Help: "Number of unrecoverable retries in the APNS connector",
	})
)

func init() {
	prometheus.MustRegister(
		pSentMessages,
		pSendErrors,
		pResponseErrors,
		pResponseInternalErrors,
		pResponseRegistrationErrors,
		pResponseOtherErrors,
		pSendNetworkErrors,
		pSendRetryCloseTLS,
		pSendRetryUnrecoverable,
	)
}
