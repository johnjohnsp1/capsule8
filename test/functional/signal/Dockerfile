FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

COPY main.c /
WORKDIR /

RUN make main

# init process can't raise signals, so use shell form of CMD
CMD "/main"
