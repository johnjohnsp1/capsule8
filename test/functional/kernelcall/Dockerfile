FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install netcat-openbsd && \
    apt-get clean

WORKDIR /kernelcall

COPY testdata/*.txt ./

CMD cat goodbye.txt hello.txt
