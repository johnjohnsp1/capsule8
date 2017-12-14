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

# Test build flags
GOTESTFLAGS+=

# Test execution flags
TESTFLAGS+=

# 'docker run' flags
DOCKERRUNFLAGS+=

# Need to use clang instead of gcc for -msan, specify its path here
CLANG=clang

# Needed to regenerate code from protos
PROTOC_GEN_GO=${GOPATH}/bin/protoc-gen-go
PROTO_INC=-I../:vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis 

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
	-u $(shell id -u):$(shell getent group docker | cut -d: -f3)        \
	-v "$$(pwd):/go/src/$(PKG)"                                         \
	-v /var/run/docker.sock:/var/run/docker.sock:ro                     \
	-w /go/src/$(PKG)                                                   \
	$(BUILD_IMAGE)

#
# Docker flags to use to run the capsule8 container
#
DOCKER_RUN=docker run                                                       \
	--privileged                                                        \
	$(DOCKERRUNFLAGS)                                                   \
	--rm                                                                \
	-v /proc:/var/run/capsule8/proc/:ro                                 \
	-v /sys/kernel/debug:/sys/kernel/debug                              \
	-v /sys/fs/cgroup:/sys/fs/cgroup                                    \
	-v /var/lib/docker:/var/lib/docker:ro                               \
	-v /var/run/capsule8:/var/run/capsule8                              \
	-v /var/run/docker:/var/run/docker:ro                               \
	$(CONTAINER_IMAGE)

#
# Docker flags to use to run the functional test container
#
DOCKER_RUN_FUNCTIONAL_TEST=docker run                                       \
	-v /var/run/capsule8:/var/run/capsule8                              \
	-v /var/run/docker.sock:/var/run/docker.sock                        \
	$(FUNCTIONAL_TEST_IMAGE)

.PHONY: all api ci ci_shell builder build_image container load save run     \
	shell static dist check test test_verbose test_all test_msan        \
	test_race test_functional build_test_functional_image               \
	run_test_functional_image clean
	run_test_functional_image run_background clean

#
# Default target: build all executables
#
all: $(BINS)

api: ../capsule8/api/v0/*.proto
        # Compile grpc and gateway stubs
	protoc --plugin=protoc-gen-go=$(PROTOC_GEN_GO) \
		--go_out=plugins=grpc:.. \
		$(PROTO_INC) \
		$?

#
# Build all container images
#
containers: container_image functional_test_image

#
# Default CI target
#
ci: | clean builder build_image
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

container_image: container

load: capsule8-$(VERSION).tar
	docker load -i $<

save: capsule8-$(VERSION).tar

capsule8-$(VERSION).tar: container_image
	docker save -o $@ $(CONTAINER_IMAGE)

run: DOCKERRUNFLAGS+=-ti
run: container
	$(DOCKER_RUN)

run_background: DOCKERRUNFLAGS+=-d
run_background: container
	$(DOCKER_RUN)

#
# Run an interactive shell within the docker container with the
# required ports and mounts. This is useful for debugging and testing
# the environment within the continer.
#
shell: DOCKERRUNFLAGS+=-ti
shell: container
	$(DOCKER_RUN) /bin/sh

#
# Build all executables as static executables
#
static:
	CGO_ENABLED=0 GOBUILDFLAGS=-a $(MAKE) -B $<

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
	go test $(GOTESTFLAGS) $(SRC) $(TESTFLAGS)

test_verbose: GOTESTFLAGS+=-v
test_verbose:
	go test $(GOTESTFLAGS) $(SRC) $(TESTFLAGS)

#
# Run all tests
#
test_all: test test_msan test_race

#
# Run all unit tests in pkg/ under memory sanitizer
#
test_msan: GOTESTFLAGS+=-msan
test_msan:
	CC=${CLANG} go test $(GOTESTFLAGS) $(SRC) $(TESTFLAGS)

#
# Run all unit tests in pkg/ under race detector
#
test_race: GOTESTFLAGS+=-race
test_race:
	go test $(GOTESTFLAGS) $(SRC) $(TESTFLAGS)

#
# Run functional test suite
#
test_functional:
	go test ./test/functional $(GOTESTFLAGS)

test/functional/functional.test: test/functional/*.go
	CGO_ENABLED=0 go test $(GOTESTFLAGS) -c -o $@ ./test/functional

#
# Build docker image for the functional test suite
#
functional_test_image: test/functional/functional.test
	docker build ./test/functional
	$(eval FUNCTIONAL_TEST_IMAGE=$(shell docker build -q ./test/functional))

#
# Run docker image for the functional test suite
#
run_functional_test: functional_test_image
	$(DOCKER_RUN_FUNCTIONAL_TEST) $(TESTFLAGS)

clean:
	rm -rf $(BINS)
