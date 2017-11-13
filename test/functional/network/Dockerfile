FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install netcat-openbsd && \
    apt-get clean

WORKDIR /network

COPY network.sh ./

EXPOSE 1234 12345

CMD sleep 1; ./network.sh 1234; ./network.sh 12345
