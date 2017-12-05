.PHONY: run_image
run_image: build_image
	docker run                                                             \
		-it                                                            \
		--privileged                                                   \
		--publish 8484:8484                                            \
		--rm                                                           \
		-v /proc:/var/run/capsule8/mnt/proc/:ro                        \
		-v /sys:/var/run/capsule8/mnt/sys:ro                           \
		-v /sys/fs/cgroup:/var/run/capsule8/mnt/sys/fs/cgroup          \
		-v /sys/kernel/debug:/var/run/capsule8/mnt/sys/kernel/debug    \
		-v /var/lib/capsule8:/var/lib/capsule8                         \
		-v /var/lib/docker:/var/lib/docker:ro                          \
		-v /var/run/capsule8:/var/run/capsule8                         \
		-v /var/run/docker:/var/run/docker:ro                          \
		-v /var/run/docker.sock:/var/run/docker.sock                   \
		$(shell docker build -q .)

.PHONY: build_image
build_image: functional.test
	docker build .

functional.test:
	CGO_ENABLED=0 GOBUILDFLAGS=-a go test -c -o $@ .

.PHONY: clean
clean:
	$(RM) *.test
