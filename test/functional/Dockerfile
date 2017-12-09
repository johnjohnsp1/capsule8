FROM alpine

# Install docker
RUN apk update && \
	apk upgrade && \
	apk --no-cache add docker

WORKDIR /functional

ADD . ./

RUN mkdir -p /var/lib/capsule8/log

# Log to files at v-level 2 so that full event-level trace is saved to container
ENTRYPOINT ["./functional.test", "-logtostderr=false", "-log_dir=/var/lib/capsule8/log", "-v=2"]
