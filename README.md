![Capsule8](docs/images/capsule8.png?raw=true "Capsule8 logo") 
==============================================================

Capsule8 performs advanced behavioral monitoring for cloud-native,
containers, and traditional Linux-based servers. This repository
contains the open-source components of the Capsule8 platform,
including the Sensor, example API client code, and command-line
interface.

## Status

Capsule8 is current in alpha and under active development. The gRPC
API is v0 and subject to change at any time, but we strive to keep the
sample code in `examples/` updated with any changes to the API.

## Docs

## Quickstart

For a quick demonstration of the capsule8 sensor's capabilities, start
up three terminal sessions to run the sensor, example telemetry
client, and target container as described below.

### Start the sensor

Start the Sensor and listen for API clients on the default local unix
socket:

```
$ make
$ sudo ./bin/sensor
I1004 18:55:23.111094    9617 version.go:33] Starting sensor (0.0.0+9b149e4)
I1004 18:55:23.111736    9617 sensor.go:121] Starting servers...
I1004 18:55:23.111746    9617 sensor.go:135] Sensor is ready
I1004 18:55:23.111761    9617 telemetry.go:29] Serving gRPC API on unix:/var/run/capsule8/sensor.sock
[...]
```

### Subscribe to telemetry for specified container images

The example telemetry client in `examples/telemetry` provides a
demonstration of how to use the Telemetry API. It subscribes to the
following telemetry:
- process lifecycle events (`fork`, `execve`, and `exit`)
- `open(2)` system call return values
- file open events for any file name matching the glob `*foo*`
- kernel function calls to `SyS_connect` involving IPv4 addresses and
  returning the parsed out `sockaddr` structure elements
- container lifecycle events (created, running, exited, destroyed)

If the `-image <glob>` argument is specified on the command-line, then
events will be filtered to only containers running images with names
matching the specified glob.

```
$ go build examples/telemetry/main.go
$ sudo ./examples/telemetry/telemetry -image busy*
Watching for container images matching busy*
event:<id:"978eec0f976c5c5b7867d9d3793bdb89e50e7d920ce3dc816659a4861722fc56" container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:3208 sensor_monotime_nanos:208974918639853 container:<type:CONTAINER_EVENT_TYPE_CREATED name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" > > 
event:<id:"8fa21518ff0a2babaa72e6e07c760c0e6573d50787d351b57f47fd1da3606810" container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:5130 sensor_monotime_nanos:208975053166529 container:<type:CONTAINER_EVENT_TYPE_RUNNING host_pid:15217 > > 
event:<id:"b631682fc0d56ff38e992f8b8afd2e964fc3a61377055bcb5abf46fd4038f37c" process_id:"fafb8d19c3ab7b6934b1fab8354decb50f2d982351984d81bb1b0ef8e6fcf586" process_pid:15217 container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:5164 sensor_monotime_nanos:208977310484246 container_name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" syscall:<type:SYSCALL_EVENT_TYPE_EXIT id:2 ret:3 > process_tid:15217 cpu:5 > 
event:<id:"4095d65d394a23be9b574f36c92d9acb1c4ded13955e000657e5aac8570f1926" process_id:"fafb8d19c3ab7b6934b1fab8354decb50f2d982351984d81bb1b0ef8e6fcf586" process_pid:15217 container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:5170 sensor_monotime_nanos:208977310715127 container_name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" process:<type:PROCESS_EVENT_TYPE_FORK fork_child_pid:15298 > process_tid:15217 cpu:5 > 
event:<id:"065d00a4e53195b0b8abdd71a1f1abdf1d233e98656f92f750f0661b1a145e92" process_id:"2a7536fb3b396281e05e4602f546745819a34741462a8aaeff7aca514f3b2898" process_pid:15298 container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:5196 sensor_monotime_nanos:208977312517716 container_name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" process:<type:PROCESS_EVENT_TYPE_EXEC exec_filename:"/bin/ls" > process_tid:15298 cpu:7 > 
event:<id:"b7b23c45ad2c1d868be4754933bc8ad8af09ee473292a54ca1f077bd2a08c5ba" process_id:"2a7536fb3b396281e05e4602f546745819a34741462a8aaeff7aca514f3b2898" process_pid:15298 container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:5197 sensor_monotime_nanos:208977312711734 container_name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" syscall:<type:SYSCALL_EVENT_TYPE_EXIT id:2 ret:3 > process_tid:15298 cpu:7 > 
[...]
event:<id:"faf7110112131d6797f7edb75f053926f05026792da1cf72b4efed50b2cf04a2" container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:6043 sensor_monotime_nanos:208979802583554 container:<type:CONTAINER_EVENT_TYPE_EXITED name:"/nostalgic_galileo" image_id:"c30178c5239f2937c21c261b0365efcda25be4921ccb95acd63beeeb78786f27" image_name:"busybox" exit_code:1 > > 
event:<id:"ba6a5118c6314209ec317472355638106a127ccfb75c45adf00fc8c7d8c778ea" container_id:"4265413fa321921b3c6014f564811a17c9e376a17ade3e7026611ceba48ed955" sensor_id:"52fdfc072182654f163f5f0f9a621d729566c74d10037c4d7bbb0407d1e2c649" sensor_sequence_number:6044 sensor_monotime_nanos:208979811686154 container:<type:CONTAINER_EVENT_TYPE_DESTROYED > > 
```

### Generate events in a matching container

In order to generate matching telemetry, start a busybox container in another terminal:

```
$ docker run --rm -it busybox
docker run --rm -it busybox
/ # ls
bin   dev   etc   home  proc  root  sys   tmp   usr   var
/ # cat /foo
cat: can't open '/foo': No such file or directory
/ # exit
```

## Contributing

For contributing guidelines see [CONTRIBUTING](./CONTRIBUTING.md).

## FAQ

### How is this supposed to be used?

The Sensor is intended to be run on a Linux host persistently and
ideally before the host begins running application workloads. It is
designed to support API clients subscribing and unsubcribing from
telemetry dynamically to implement various security incident detection
strategies.

### What types of events can be subscribed to currently?

Container lifecycle, process lifecycle, raw system calls, file opens,
network activity, and kernel function calls.

### Kernel function calls?

You can subscribe to calls to a chosen exported function symbol and
receive telemetry events with named values of the data requested. This
data can include function call arguments, return values, register
values, and even values dereferences via offsets from any of them. For
a more detailed description of what's possible, see the Linux kernel
[kprobe docs](https://www.kernel.org/doc/Documentation/trace/kprobetrace.txt).

### What guarantees does the Sensor provide?

The Sensor provides telemetry events on a best-effort
basis. System-level events are intentionally monitored through
`perf_event_open(2)` such that an excessive volume of events causes
them to be dropped by the kernel rather than blocking the kernel as
the audit subsystem may do. This means that telemetry events, and even
some of the information within them, is "lossy" by design. We believe
that this is the right trade-off for monitoring production
environments where stability and performance are critical.
