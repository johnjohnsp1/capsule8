package filter

// Filter exports an interface for filtering events
type Filter interface {
	// FilterFunc takes an arbitrary event and returns a bool
	// representing whether or not we should include this event
	FilterFunc(interface{}) bool
	// DoFunc performs some action or side effect per arbitrary event
	DoFunc(interface{})
}
