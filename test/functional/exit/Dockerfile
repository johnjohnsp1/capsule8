FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

COPY main.c status*.c /
WORKDIR /

RUN make main status0 status1 status2

CMD /status1; /status2; /status0; /main
