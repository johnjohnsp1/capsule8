FROM alpine

RUN apk upgrade && \
	apk add --no-cache alpine-sdk

WORKDIR /stress

COPY main.c stress.sh ./

RUN make main

CMD ["./stress.sh"]
