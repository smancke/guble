package gcm

import (
	"github.com/smancke/guble/metrics"
)

var (
	mTotalSentMessages      = metrics.NewInt("guble.gcm.total_sent_messages")
	mTotalSentMessageErrors = metrics.NewInt("guble.gcm.total_sent_message_errors")
)

func resetGcmMetrics() {
	mTotalSentMessages.Set(0)
	mTotalSentMessageErrors.Set(0)
}
