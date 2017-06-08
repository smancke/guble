package sms

import "github.com/prometheus/client_golang/prometheus"

var (
	pSent = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sms_sent",
		Help: "Number of sms sent to the SMS service",
	})

	pNexmoSendErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sms_nexmo_send_errors",
		Help: "Number of errors while trying to send sms to Nexmo",
	})

	pNexmoResponseErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sms_nexmo_response_errors",
		Help: "Number of errors received from Nexmo",
	})

	pNexmoResponseInternalErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "sms_nexmo_response_internal_errors",
		Help: "Number of internal errors related to Nexmo responses",
	})
)

func init() {
	prometheus.MustRegister(
		pSent,
		pNexmoSendErrors,
		pNexmoResponseErrors,
		pNexmoResponseInternalErrors,
	)
}
