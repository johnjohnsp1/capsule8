package perf

/*

We universally obtain the time of all perf event via PERF_SAMPLE_TIME.

From perf_event_open(2):
    If  PERF_SAMPLE_TIME  is  enabled,  then a 64-bit timestamp is included.  This is
    obtained via local_clock() which is a hardware timestamp  if  available  and  the
    jiffies value if not.

This most closely corresponds to CLOCK_MONOTONIC_RAW available via clock_gettime()
for other types of events.

The clock used for perf events prior to Linux 4.1 is local_clock() (via cpu_clock()).
After 4.1, we can specify a clockid in the perf_event_attr to select a different
clock.

From http://elixir.free-electrons.com/linux/v2.6.39/source/kernel/sched_clock.c:

 cpu_clock(i) provides a fast (execution time) high resolution
 clock with bounded drift between CPUs. The value of cpu_clock(i)
 is monotonic for constant i. The timestamp returned is in nanoseconds.

 ######################### BIG FAT WARNING ##########################
 # when comparing cpu_clock(i) to cpu_clock(j) for i != j, time can #
 # go backwards !!                                                  #
 ####################################################################

 There is no strict promise about the base, although it tends to start
 at 0 on boot (but people really shouldn't rely on that).

*/
