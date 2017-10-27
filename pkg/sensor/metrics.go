package sensor

// Counters used for metrics
type MetricsCounters struct {
	// Number of events created during the sample period
	Events uint64

	// Number of subscriptions
	Subscriptions int32
}
