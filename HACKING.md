# Hacking on Capsule8

## Sensor

### Logging and Tracing

Capsule8 uses [glog](https://github.com/golang/glog) for logging and
all executables accept the `-v` and `-vmodule` flags. In general, log
levels 1-9 increase verbosity of logging while >10 increase verbosity
of event and request-specific tracing. These log levels may be set
globally with a command-line argument like `-v=9` or on a specific
file with `-vmodule=process_info=11`.

### Functional Tests

The functional tests in `test/functional` require privileges to run
the `docker` command and reach the Sensor's gRPC API endpoint so they
can be run with `sudo` if necessary:

```
$ env GOTESTFLAGS="-exec sudo" make test_functional
go test -exec sudo ./test/functional
[sudo] password for user:
[...]
```

#### Running Functional Tests on a Remote Host

The functional tests have been designed to support being run against a
remote Capsule8 API and Docker host.

```
make CAPSULE8_API_SERVER=127.0.0.1:8484 DOCKER_HOST=tcp://127.0.0.1:2375 test_functional

```

#### Debugging Functional Tests

In order to debug functional tests, first run the failing test
individually with `v=1`. This V-level causes the test to log
intermediate status information.

```
$ make GOTESTFLAGS="-test.v -test.parallel 1 -test.run Crash -args -v=1" test_functional 2>test.out
=== RUN   TestCrash
=== RUN   TestExit
=== RUN   TestSignal
=== RUN   TestCrash/buildContainer
=== RUN   TestCrash/runTelemetryTest
--- PASS: TestCrash (1.50s)
    --- PASS: TestCrash/buildContainer (0.11s)
    --- PASS: TestCrash/runTelemetryTest (1.39s)
=== RUN   TestSignal/buildContainer
=== RUN   TestSignal/runTelemetryTest
--- FAIL: TestSignal (10.15s)
    --- PASS: TestSignal/buildContainer (0.15s)
    --- FAIL: TestSignal/runTelemetryTest (10.00s)
    	telemetry_test.go:90: rpc error: code = DeadlineExceeded desc = context deadline exceeded
	telemetry_test.go:101: Couldn't run telemetry tests
=== RUN   TestExit/buildContainer
=== RUN   TestExit/runTelemetryTest
--- FAIL: TestExit (10.13s)
    --- PASS: TestExit/buildContainer (0.13s)
    --- FAIL: TestExit/runTelemetryTest (10.00s)
    	telemetry_test.go:90: rpc error: code = DeadlineExceeded desc = context deadline exceeded
	telemetry_test.go:101: Couldn't run telemetry tests
FAIL
```

```
$ cat test.out
I1016 13:31:00.967011    1348 crash_test.go:94] containerID = feefae314519bfdc442fc6bb4b223a959ca6300483d14fb9e43478c7454f604c
I1016 13:31:01.125155    1348 crash_test.go:120] processID = da9de705bbd204fc7a0ffa247fc973daf0b80234a0ac382ab3ce74f4fe1c0814
I1016 13:31:02.126712    1348 crash_test.go:145] processExited = true
I1016 13:31:02.277126    1348 crash_test.go:107] containerExited = true
```

Additional information can be obtained by running the test with `v=2`,
which causes the test to log full events recieved. This can be a lot
of information, so it's highly recommended that stderr be redirected
to a file.

```
$ make GOTESTFLAGS="-test.v -test.parallel 1 -args -v=2" test_functional 2>test.out
```

### Performance Tests

In order to measure performance improvements or regressions of the
Sensor, there is a simple macro benchmark in `test/benchmark`. The
benchmark assumes that only one Docker container is running at a time,
so make sure to perform this testing when no other Docker containers
are running.

Start the benchmark in one window, as shown below. It will print out
the number of events received on the subscription as well as the
`getrusage(2)` delta between when a container starts and stops:

```
$ cd test/benchmark
$ go build .
$ sudo ./benchmark 
fa29d62433bf493a2c494a0cb2ff90aa2372b4b00f47cfc0903c55a67b7479ed Events:73606 avg_user_us_per_event:249 avg_sys_us_per_event:105 {Events:73606 Subscriptions:1} {Utime:{Sec:18 Usec:389000} Stime:{Sec:7 Usec:767000} Maxrss:16544 Ixrss:0 Idrss:0 Isrss:0 Minflt:1055 Majflt:0 Nswap:0 Inblock:0 Oublock:8 Msgsnd:0 Msgrcv:0 Nsignals:0 Nvcsw:613202 Nivcsw:43554}
```

In order to generate a large number of events, you can use the kernel
compile container in `test/benchmark/kernel_compile`:

```
$ cd test/benchmark/kernel_compile
$ make
[...]
840.79user 64.07system 2:11.97elapsed 685%CPU (0avgtext+0avgdata 146052maxresident)k
0inputs+28248outputs (0major+24220864minor)pagefaults 0swaps
```

### Profiling

Profiling may be enabled in the Sensor by setting the
`CAPSULE8_PROFILING_ADDR` environment variable to the TCP socket to
listen on (e.g. ":6060"). This enables the most types of profiling via
`go tool pprof` to be used against a running Sensor.

The benchmark test `-profile` option enables the Go 
[profiling HTTP server](https://golang.org/pkg/net/http/pprof/) on
port 6060 and configures Go runtime parameters to enable goroutine
blocking and mutex contention tracing (which negatively impact
performance).  This allows `go tool pprof` commands to be used to
collect useful traces.


CPU profile:
```
$ go tool pprof http://localhost:6060/debug/pprof/profile
```

Heap profile:
```
$ go tool pprof http://localhost:6060/debug/pprof/heap
```

Goroutine blocking profile:
```
$ go tool pprof http://localhost:6060/debug/pprof/block
```

Mutex contention profile:
```
$ go tool pprof http://localhost:6060/debug/pprof/mutex
```

