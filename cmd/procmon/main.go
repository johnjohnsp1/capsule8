package main

import (
	"log"
	"os"

	"fmt"

	"github.com/capsule8/reactive8/pkg/perf"
)

func main() {
	eventAttr := &perf.EventAttr{
		Type:            perf.PERF_TYPE_TRACEPOINT,
		Config:          120, // syscalls:sys_enter_exit_group
		SampleType:      perf.PERF_SAMPLE_TID | perf.PERF_SAMPLE_TIME | perf.PERF_SAMPLE_CPU,
		SampleIDAll:     true,
		Comm:            true,
		CommExec:        true,
		Task:            true,
		SamplePeriod:    1,
		Watermark:       true,
		WakeupWatermark: 1,
	}

	p, err := perf.New(*eventAttr)
	if err != nil {
		log.Fatal(err)
	}

	onRecord := func(ev *perf.Event, err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return
		}

		switch ev.Data.(type) {
		case *perf.Fork:
			time := ev.Time
			fmt.Printf("%d FORK(%v)\n", time, ev)
		case *perf.Comm:
			time := ev.Time
			fmt.Printf("%d COMM(%v)\n", time, ev)
		case *perf.Exit:
			time := ev.Time
			fmt.Printf("%d EXIT(%v)\n", time, ev)
		default:
			fmt.Printf("%d\n", ev.Type)
		}
	}

	p.Enable()
	p.Run(onRecord)
}
