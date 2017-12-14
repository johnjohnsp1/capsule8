## 0.2.0 (Dec 14, 2017)

BACKWARDS INCOMPATIBILITIES:

  * Event filtering changed in ([#55](https://github.com/capsule8/capsule8/pull/55)) with updates to the underlying API definitions.

FEATURES:

  * Default to system wide event monitor even when running in container ([#62](https://github.com/capsule8/capsule8/pull/62))
  * Use single event monitor for all subscriptions ([#61](https://github.com/capsule8/capsule8/pull/61))
  * Use expression filtering from API for event based filtering ([#55](https://github.com/capsule8/capsule8/pull/55))
  * Add process credential tracking ([#57](https://github.com/capsule8/capsule8/pull/57))

IMPROVEMENTS:

  * Add copyright statement and license to all source files ([#76](https://github.com/capsule8/capsule8/pull/76))
  * Add kinesis telemetry ingestor example ([#71](https://github.com/capsule8/capsule8/pull/71))
  * Refactor network functional tests ([#72](https://github.com/capsule8/capsule8/pull/72))
  * Import telemetry API definitions that used to be vendored, directly to this respository ([#70](https://github.com/capsule8/capsule8/pull/70))
  * Create docker image for functional testing ([#63](https://github.com/capsule8/capsule8/pull/63))
  * Separate our sensor start and stop logic ([#68](https://github.com/capsule8/capsule8/pull/68))
  * Update service constructors to pass sensor reference ([#66](https://github.com/capsule8/capsule8/pull/66))
  * Add functional testing for network, syscall and kernelcall events ([#54](https://github.com/capsule8/capsule8/pull/54))
  * Update expression use to programmatically create expression trees ([#60](https://github.com/capsule8/capsule8/pull/60))
  * Improve and add to unit testing of /proc/[pid]/cgroup parsing ([#53](https://github.com/capsule8/capsule8/pull/53))
  * Remove sleeps in functional tests to make them more demanding ([#52](https://github.com/capsule8/capsule8/pull/52))
  * Use functions to configure the new event monitor input ([#37](https://github.com/capsule8/capsule8/pull/37))
  * Refactoring to set up the single event monitor work ([#31](https://github.com/capsule8/capsule8/pull/31))

BUG FIXES:

  * Fix identification of container IDs in a kubernetes environment ([#69](https://github.com/capsule8/capsule8/pull/69))
  * Fix duplicate events off by one error ([#64](https://github.com/capsule8/capsule8/pull/64))
  * Fix container information identification from Docker versions before 1.13.0 ([#56](https://github.com/capsule8/capsule8/pull/56))
  * Fix 'args' type information handling in various kernel versions ([#51](https://github.com/capsule8/capsule8/pull/51))


## 0.1.1 (Dec 14, 2017)

BACKWARDS INCOMPATIBILITIES:

  None

FEATURES:

  None

IMPROVEMENTS:

  None

BUG FIXES:

  * Fix container information identification from Docker versions before 1.13.0 ([#65](https://github.com/capsule8/capsule8/pull/65))
  * Fix identification of container IDs in a kubernetes environment ([#67](https://github.com/capsule8/capsule8/pull/67))
