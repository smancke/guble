package router

import (
	"github.com/cosminrentea/gobbler/server/metrics"
)

var (
	mTotalSubscriptionAttempts                 = metrics.NewInt("router.total_subscription_attempts")
	mTotalDuplicateSubscriptionsAttempts       = metrics.NewInt("router.total_subscription_attempts_duplicate")
	mTotalSubscriptions                        = metrics.NewInt("router.total_subscriptions")
	mTotalUnsubscriptionAttempts               = metrics.NewInt("router.total_unsubscription_attempts")
	mTotalInvalidTopicOnUnsubscriptionAttempts = metrics.NewInt("router.total_unsubscription_attempts_invalid_topic")
	mTotalInvalidUnsubscriptionAttempts        = metrics.NewInt("router.total_unsubscription_attempts_invalid")
	mTotalUnsubscriptions                      = metrics.NewInt("router.total_unsubscriptions")
	mCurrentSubscriptions                      = metrics.NewInt("router.current_subscriptions")
	mCurrentRoutes                             = metrics.NewInt("router.current_routes")
	mTotalMessagesIncoming                     = metrics.NewInt("router.total_messages_incoming")
	mTotalMessagesIncomingBytes                = metrics.NewInt("router.total_messages_bytes_incoming")
	mTotalMessagesStoredBytes                  = metrics.NewInt("router.total_messages_bytes_stored")
	mTotalMessagesRouted                       = metrics.NewInt("router.total_messages_routed")
	mTotalOverloadedHandleChannel              = metrics.NewInt("router.total_overloaded_handle_channel")
	mTotalMessagesNotMatchingTopic             = metrics.NewInt("router.total_messages_not_matching_topic")
	mTotalMessageStoreErrors                   = metrics.NewInt("router.total_errors_message_store")
	mTotalDeliverMessageErrors                 = metrics.NewInt("router.total_errors_deliver_message")
	mTotalNotMatchedByFilters                  = metrics.NewInt("router.total_not_matched_by_filters")
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
	mTotalNotMatchedByFilters.Set(0)
}
