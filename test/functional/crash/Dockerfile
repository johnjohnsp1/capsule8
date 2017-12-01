FROM alpine

RUN apk upgrade && \
	apk add --no-cache alpine-sdk

COPY main.c /

WORKDIR /

RUN make main

CMD ["/main"]
