package router

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	pSubscriptionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_subscription_attempts",
		Help: "Number of subscription attempts in router",
	})

	pDuplicateSubscriptionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_duplicate_subscription_attempts",
		Help: "Number of duplicate subscription attempts in router",
	})

	pTotalSubscriptions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_total_subscriptions",
		Help: "Total number of subscriptions in router",
	})

	pUnsubscriptionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_total_unsubscription_attempts",
		Help: "Number of unsubscriptions attempts in router",
	})

	pInvalidTopicOnUnsubscriptionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_total_unsubscription_attempts_invalid_topic",
		Help: "Number of unsubscriptions attempts having invalid topic in router",
	})

	pInvalidUnsubscriptionAttempts = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_total_unsubscription_attempts_invalid",
		Help: "Number of invalid subscription attempts in router",
	})

	pTotalUnsubscriptions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_total_unsubscriptions",
		Help: "Total number of unsubscriptions in router",
	})

	pSubscriptions = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "router_current_subscriptions",
		Help: "Number of active subscriptions in router",
	})

	pRoutes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "router_current_routes",
		Help: "Number of active routes in router",
	})

	pMessagesIncoming = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_messages_incoming",
		Help: "Number of incoming messages in router",
	})

	pMessagesIncomingBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_messages_bytes_incoming",
		Help: "Number of bytes in incoming messages in router",
	})

	pMessagesStoredBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_messages_bytes_stored",
		Help: "Number of bytes stored from incoming messages in router",
	})

	pMessagesRouted = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_messages_routed",
		Help: "Number of messages routed in router",
	})

	pOverloadedHandleChannel = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_overloaded_handle_channel",
		Help: "Number of overloaded handle-channel occurrences in router",
	})

	pMessagesNotMatchingTopic = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_messages_not_matching_topic",
		Help: "Number of messages not matching a topic in router",
	})

	pMessageStoreErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_errors_message_store",
		Help: "Number of errors related to the message-store in the router",
	})

	pDeliverMessageErrors = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_errors_deliver_message",
		Help: "Number of errors when trying to deliver the message in router",
	})

	pNotMatchedByFilters = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "router_not_matched_by_filters",
		Help: "Number of messages not matched by any filters in the router",
	})
)

func init() {
	prometheus.MustRegister(
		pSubscriptionAttempts,
		pDuplicateSubscriptionAttempts,
		pTotalSubscriptions,
		pUnsubscriptionAttempts,
		pInvalidTopicOnUnsubscriptionAttempts,
		pInvalidUnsubscriptionAttempts,
		pTotalUnsubscriptions,
		pSubscriptions,
		pRoutes,
		pMessagesIncoming,
		pMessagesIncomingBytes,
		pMessagesStoredBytes,
		pMessagesRouted,
		pOverloadedHandleChannel,
		pMessagesNotMatchingTopic,
		pMessageStoreErrors,
		pDeliverMessageErrors,
		pNotMatchedByFilters,
	)
}
