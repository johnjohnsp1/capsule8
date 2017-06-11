package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/capsule8/reactive8/pkg/syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr,
			"usage: %s PID [ syscall_number ...]\n", os.Args[0])
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Couldn't parse pid %s\n", os.Args[1])

		os.Exit(1)
	}

	syscallNrs := make([]uint, len(os.Args)-2)

	if len(os.Args) > 2 {
		for i := 2; i < len(os.Args); i++ {
			v, err := strconv.Atoi(os.Args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"Couldn't parse syscall number %s\n", os.Args[i])

				os.Exit(1)

			}
			syscallNrs[i-2] = uint(v)
		}

	}

	syscalls, err := syscall.EventsForPid(pid, syscallNrs...)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Couldn't create syscall event stream: %v\n", err)

		os.Exit(1)
	}

	go func() {
		signals := make(chan os.Signal)
		signal.Notify(signals, os.Interrupt)
		<-signals
		syscalls.Close()
	}()

	ticker := time.Tick(1 * time.Second)
	numEvents := 0

	for {
		select {
		case <-ticker:
			fmt.Fprintf(os.Stderr, "Events/s: %d\n", numEvents)
			numEvents = 0

		case e, ok := <-syscalls.Data:
			if ok {
				numEvents++
				fmt.Printf("Event: %x\n", e)
			} else {
				fmt.Fprintf(os.Stderr, "Stream closed.\n")
				os.Exit(0)
			}
		}
	}

}
