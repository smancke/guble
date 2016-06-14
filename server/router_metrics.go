package server

import (
	"github.com/smancke/guble/metrics"
)

var (
	mTotalSubscriptionAttempts                 = metrics.NewInt("guble.router.total_subscription_attempts")
	mTotalDuplicateSubscriptionsAttempts       = metrics.NewInt("guble.router.total_subscription_attempts_duplicate")
	mTotalSubscriptions                        = metrics.NewInt("guble.router.total_subscriptions")
	mTotalUnsubscriptionAttempts               = metrics.NewInt("guble.router.total_unsubscription_attempts")
	mTotalInvalidTopicOnUnsubscriptionAttempts = metrics.NewInt("guble.router.total_unsubscription_attempts_invalid_topic")
	mTotalInvalidUnsubscriptionAttempts        = metrics.NewInt("guble.router.total_unsubscription_attempts_invalid")
	mTotalUnsubscriptions                      = metrics.NewInt("guble.router.total_unsubscriptions")
	mCurrentSubscriptions                      = metrics.NewInt("guble.router.current_subscriptions")
	mCurrentRoutes                             = metrics.NewInt("guble.router.current_routes")
	mTotalMessagesIncoming                     = metrics.NewInt("guble.router.total_messages_incoming")
	mTotalMessagesIncomingBytes                = metrics.NewInt("guble.router.total_messages_bytes_incoming")
	mTotalMessagesStoredBytes                  = metrics.NewInt("guble.router.total_messages_bytes_stored")
	mTotalMessagesRouted                       = metrics.NewInt("guble.router.total_messages_routed")
	mTotalOverloadedHandleChannel              = metrics.NewInt("guble.router.total_overloaded_handle_channel")
	mTotalMessagesNotMatchingTopic             = metrics.NewInt("guble.router.total_messages_not_matching_topic")
	mTotalMessageStoreErrors                   = metrics.NewInt("guble.router.total_errors_message_store")
	mTotalDeliverMessageErrors                 = metrics.NewInt("guble.router.total_errors_deliver_message")
)

func resetRouterMetrics() {
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
	mTotalMessagesIncomingBytes.Set(0)
	mTotalMessagesStoredBytes.Set(0)
}
