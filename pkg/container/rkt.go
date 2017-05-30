package container

type rktPodState uint

const (
	// From: https://coreos.com/rkt/docs/latest/devel/pod-lifecycle.html
	rktPodUnknown rktPodState = iota
	rktPodPrepare
	rktPodRun
	rktPodExitedGarbage
	rktPodGarbage
)
