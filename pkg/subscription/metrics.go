package subscription

var (
	// Current metrics counters, will be incremented
	Metrics MetricsCounters
)

// Counters used for metrics
type MetricsCounters struct {
	// Number of events created during the sample period
	Events uint64

	// Number of subscriptions
	Subscriptions int32
}
