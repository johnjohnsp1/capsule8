FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

WORKDIR /network

COPY main.c ./

EXPOSE 1234 12345

RUN cc main.c -o main -pthread

CMD ./main 1234; ./main 12345
