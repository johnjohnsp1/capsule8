FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

WORKDIR /stress

COPY test-proc.c stress.sh ./

RUN make test-proc

CMD ["./stress.sh"]
