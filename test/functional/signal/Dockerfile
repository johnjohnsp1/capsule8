FROM alpine

RUN apk upgrade && \
	apk add --no-cache alpine-sdk

COPY main.c /
WORKDIR /

RUN make main

# init process can't raise signals, so use shell form of CMD
CMD "/main"
