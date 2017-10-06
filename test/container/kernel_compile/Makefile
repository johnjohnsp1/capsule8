#
# Run a timed kernel unpack, build, and clean on a tmpfs
#
IMAGE_TAG=capsule8-test-kernel_compile

run: build
	docker run --rm --tmpfs /usr/src:size=2147483648,exec -it $$(docker build -t $(IMAGE_TAG) . -q)

build: Dockerfile
	docker build -t $(IMAGE_TAG) .
