# Go import path of this repo
PKG=github.com/capsule8/capsule8
REPO=$(shell basename $(shell readlink -f .))

#
# SemVer 2.0 version string: (X.Y.Z-pre-release-identifier+build.metadata)
#
TAG=$(shell git describe --tags --abbrev=0 2>/dev/null)
SHA=$(shell git describe --match=NeVeRmAtCh --always --abbrev=7 --dirty)

ifeq ($(TAG),)
	VERSION=0.0.0+$(SHA)
else
	VERSION=$(TAG)+$(SHA)
endif

# Automated build unique identifier (if any)
BUILD=$(shell echo ${BUILD_ID})

# 'go build' flags
GOBUILDFLAGS+=-ldflags "-X $(PKG)/pkg/version.Version=$(VERSION) -X $(PKG)/pkg/version.Build=$(BUILD)"
GOVETFLAGS+=-shadow
GOTESTFLAGS+=

# Need to use clang instead of gcc for -msan, specify its path here
CLANG=clang

# All command-line executables in cmd/
CMDS=$(notdir $(wildcard ./cmd/*))
BINS=$(patsubst %,bin/%,$(CMDS))

# All source directories that need to be checked, compiled, tested, etc.
SRC=./cmd/... ./pkg/... ./examples/...

#
# Docker flags to use in CI
#
DOCKER_RUN_CI=docker run                                                    \
	--network host                                                      \
	-ti                                                                 \
	--rm                                                                \
	-v "$$(pwd):/go/src/$(PKG)"                                         \
	-v /var/run/docker.sock:/var/run/docker.sock:ro                     \
	-w /go/src/$(PKG)                                                   \
	$(BUILD_IMAGE)

.PHONY: all ci ci_shell builder build_image container load save run shell \
	static dist check test test_verbose test_all test_msan test_race  \
	test_functional clean

#
# Default target: build all executables
#
all: $(BINS)

#
# Default CI target
#
ci: | builder build_image
	$(DOCKER_RUN_CI) /bin/sh -c "                                           \
		./build/build.sh &&                                             \
		./build/test.sh                                                 \
	    "

ci_shell: | builder build_image
	$(DOCKER_RUN_CI) /bin/sh

builder: build/Dockerfile
	docker build build/

build_image:
	$(eval BUILD_IMAGE=$(shell docker build -q build/))

container: Dockerfile static
	docker build --build-arg vcsref=$(SHA) --build-arg version=$(VERSION) .
	$(eval CONTAINER_IMAGE=$(shell docker build -q .))

load: capsule8-$(VERSION).tar
	docker load -i

save: capsule8-$(VERSION).tar

capsule8-$(VERSION).tar: container
	docker save -o $@ $(CONTAINER_IMAGE)

run: container
	docker run --rm -it                                                    \
		--privileged                                                   \
		--publish 8484:8484                                            \
		--volume=/proc:/var/run/capsule8/proc/:ro                      \
		--volume=/sys/kernel/debug:/sys/kernel/debug                   \
		--volume=/sys/fs/cgroup:/sys/fs/cgroup                         \
		--volume=/var/lib/docker:/var/lib/docker:ro                    \
		--volume=/var/run/docker:/var/run/docker:ro                    \
		$(CONTAINER_IMAGE)

#
# Run an interactive shell within the docker container with the
# required ports and mounts. This is useful for debugging and testing
# the environment within the continer.
#
shell: container
	docker run --rm -it                                                    \
		--privileged                                                   \
		--publish 8484:8484                                            \
		--volume=/proc:/var/run/capsule8/proc/:ro                      \
		--volume=/sys/kernel/debug:/sys/kernel/debug                   \
		--volume=/sys/fs/cgroup:/sys/fs/cgroup                         \
		--volume=/var/lib/docker:/var/lib/docker:ro                    \
		--volume=/var/run/docker:/var/run/docker:ro                    \
		$(CONTAINER_IMAGE) /bin/sh

#
# Build all executables as static executables
#
static:
	CGO_ENABLED=0 GOBUILDFLAGS=-a $(MAKE) $<

#
# Make a distribution tarball
#
dist: static
	tar -czf capsule8-$(VERSION).tar.gz bin/ ./examples/ ./vendor/

#
# Pattern rules to allow 'make foo' to build ./cmd/foo or ./test/cmd/foo (whichever exists)
#
bin/% : cmd/% cmd/%/*.go
	go build $(GOBUILDFLAGS) -o $@ ./$<

#
# Check that all sources build successfully, gofmt, go vet, golint, etc)
#
check:
	echo "--- Checking source code formatting"
	find ./cmd ./pkg ./examples -name '*.go' | xargs gofmt -d
	echo "--- Checking that all sources build"
	go build $(SRC)
	echo "--- Checking that all sources vet clean"
	go vet $(GOVETFLAGS) $(SRC)
	echo "--- Checking sources for lint"
	golint $(SRC)

#
# Run all unit tests quickly
#
test: GOTESTFLAGS+=-cover
test:
	go test $(SRC) $(GOTESTFLAGS) 

test_verbose: GOTESTFLAGS+=-v
test_verbose:
	go test $(SRC) $(GOTESTFLAGS)

#
# Run all tests
#
test_all: test test_msan test_race

#
# Run all unit tests in pkg/ under memory sanitizer
#
test_msan: GOTESTFLAGS+=-msan
test_msan:
	CC=${CLANG} go test $(SRC) $(GOTESTFLAGS) 

#
# Run all unit tests in pkg/ under race detector
#
test_race: GOTESTFLAGS+=-race
test_race:
	go test $(SRC) $(GOTESTFLAGS)

#
# Run functional test suite
#
test_functional:
	go test ./test/functional $(GOTESTFLAGS)

clean:
	rm -rf ./bin $(CMDS)
