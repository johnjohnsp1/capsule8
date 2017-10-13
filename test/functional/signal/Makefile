#
# Run a timed kernel unpack, build, and clean on a tmpfs
#
IMAGE_TAG=capsule8-test-crash

run: build
	docker run --rm -it $$(docker build -t $(IMAGE_TAG) . -q)

build: Dockerfile killme.c
	docker build -t $(IMAGE_TAG) .
