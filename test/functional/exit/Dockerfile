FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

COPY exitfail.c /
WORKDIR /

RUN make exitfail

CMD "/exitfail"
