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

BUILD_IMAGE ?= golang:1.9-alpine

# we export the variables so you only have to update the version here in the top level Makefile
LDFLAGS=-ldflags "-X $(PKG)/pkg/version.Version=$(VERSION) -X $(PKG)/pkg/version.Build=$(BUILD)"

# Need to use clang instead of gcc for -msan, specify its path here
CLANG=clang

# All command-line executables in cmd/
CMDS=$(notdir $(wildcard ./cmd/*))

#
# Default target: build all executables
#
all: $(CMDS)

#
# Default CI target
#
ci:
	@docker run                                                             \
	    -ti                                                                 \
	    --rm                                                                \
	    -v "$$(pwd):/go/src/$(PKG)"                                         \
	    -w /go/src/$(PKG)                                                   \
	    $(BUILD_IMAGE)                                                      \
	    /bin/sh -c "                                                        \
		apk add -U make &&                                              \
		make check && make test_verbose                                 \
	    "

#
# Build all executables as static executables
#
static:
	CGO_ENABLED=0 $(MAKE) $(CMDS)

#
# Make a distribution tarball
#
dist: static
	tar -czf capsule8-$(VERSION).tar.gz bin/ ./examples/ ./vendor/

#
# Pattern rules to allow 'make foo' to build ./cmd/foo or ./test/cmd/foo (whichever exists)
#
% : cmd/% cmd/%/*.go
	go build $(LDFLAGS) -o bin/$@ ./$<

#
# Check that all main packages build successfully
#
check:
	go build ./cmd/... ./examples/...
	go vet ./cmd/... ./pkg/... ./examples/...

#
# Run an interactive busybox container with top-level directory mounted into
# it. This is useful for testing built command-line executables within a
# container.
#
# NB:
# - We mount most host directories read-only into the same paths inside the
#   container so that most sensor code can be written obvlivious to its
#   containerized or non-containerized state.
# - Host /sys must be mounted on /sys in container or else /sys/fs/cgroups will
#   be empty.
# - Mounting /proc on /proc is not allowed by OCI, so we mount it on /host/proc
#
contain:
	docker run \
	--volume=$(shell pwd):/$(REPO):ro -w /$(REPO) \
	--volume=/proc:/host/proc/:ro \
	--volume=/sys:/sys:ro \
	--volume=/sys/kernel/debug/tracing:/sys/kernel/debug/tracing \
	--volume=/var/run/capsule8:/var/run/capsule8 \
	--volume=/var/lib/docker:/var/lib/docker:ro \
	--volume=/var/run/docker:/var/run/docker:ro \
	--volume=/var/run/docker.sock:/var/run/docker.sock:ro \
	--privileged --rm -it busybox

#
# Run all unit tests quickly
#
.PHONY: test
test:
	go test ./cmd/... ./pkg/...

test_verbose:
	go test -v ./cmd/... ./pkg/...

#
# Run all tests
#
test_all: test test_msan test_race

#
# Run all unit tests in pkg/ under memory sanitizer
#
test_msan:
	CC=${CLANG} go test -msan ./cmd/... ./pkg/...

#
# Run all unit tests in pkg/ under race detector
#
test_race:
	go test -race ./cmd/... ./pkg/...

clean:
	rm -rf ./bin $(CMDS)
