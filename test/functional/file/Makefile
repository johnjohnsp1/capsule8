#
# Run a timed kernel unpack, build, and clean on a tmpfs
#
IMAGE_TAG=capsule8-test-open

run: build
	docker run --rm -it $$(docker build -t $(IMAGE_TAG) . -q)

build: Dockerfile filedata.h init.c open.c
	docker build -t $(IMAGE_TAG) .
