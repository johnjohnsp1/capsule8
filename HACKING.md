# Hacking on Capsule8

## Sensor Performance Testing

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

