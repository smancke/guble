package server

import (
	"expvar"
	"fmt"
	"net/http"
)

var (
	mTotalSubscriptionAttempts                 = expvar.NewInt("guble.router.total_subscription_attempts")
	mTotalDuplicateSubscriptionsAttempts       = expvar.NewInt("guble.router.total_subscription_attempts_duplicate")
	mTotalSubscriptions                        = expvar.NewInt("guble.router.total_subscriptions")
	mTotalUnsubscriptionAttempts               = expvar.NewInt("guble.router.total_unsubscription_attempts")
	mTotalInvalidTopicOnUnsubscriptionAttempts = expvar.NewInt("guble.router.total_unsubscription_attempts_invalid_topic")
	mTotalInvalidUnsubscriptionAttempts        = expvar.NewInt("guble.router.total_unsubscription_attempts_invalid")
	mTotalUnsubscriptions                      = expvar.NewInt("guble.router.total_unsubscriptions")
	mCurrentSubscriptions                      = expvar.NewInt("guble.router.current_subscriptions")
	mCurrentRoutes                             = expvar.NewInt("guble.router.current_routes")
	mTotalMessagesIncoming                     = expvar.NewInt("guble.router.total_messages_incoming")
	mTotalMessagesRouted                       = expvar.NewInt("guble.router.total_messages_routed")
	mTotalOverloadedHandleChannel              = expvar.NewInt("guble.router.total_overloaded_handle_channel")
	mTotalMessagesNotMatchingTopic             = expvar.NewInt("guble.router.total_messages_not_matching_topic")
	mTotalMessageStoreErrors                   = expvar.NewInt("guble.router.total_errors_message_store")
	mTotalDeliverMessageErrors                 = expvar.NewInt("guble.router.total_errors_deliver_message")
)

func resetMetricsRouter() {
	mTotalSubscriptionAttempts.Set(0)
	mTotalDuplicateSubscriptionsAttempts.Set(0)
	mTotalSubscriptions.Set(0)
	mTotalUnsubscriptionAttempts.Set(0)
	mTotalInvalidTopicOnUnsubscriptionAttempts.Set(0)
	mTotalUnsubscriptions.Set(0)
	mTotalInvalidUnsubscriptionAttempts.Set(0)
	mCurrentSubscriptions.Set(0)
	mCurrentRoutes.Set(0)
	mTotalMessagesIncoming.Set(0)
	mTotalMessagesRouted.Set(0)
	mTotalOverloadedHandleChannel.Set(0)
	mTotalMessagesNotMatchingTopic.Set(0)
	mTotalDeliverMessageErrors.Set(0)
	mTotalMessageStoreErrors.Set(0)
}

func expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprintf(w, "{\n")
	first := true
	expvar.Do(func(kv expvar.KeyValue) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}
