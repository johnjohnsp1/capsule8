FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install netcat-openbsd && \
    apt-get clean

WORKDIR /network

COPY network.sh ./

CMD ./network.sh
