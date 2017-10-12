FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

COPY crash.c /
WORKDIR /

RUN make crash

CMD "/crash"
