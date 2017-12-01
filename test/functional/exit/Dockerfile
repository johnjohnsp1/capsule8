FROM alpine

RUN apk upgrade && \
	apk add --no-cache alpine-sdk

COPY main.c status*.c /
WORKDIR /

RUN make main status0 status1 status2

CMD /status1; /status2; /status0; /main
