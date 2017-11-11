FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

WORKDIR /stress

COPY main.c stress.sh ./

RUN make main

CMD ["./stress.sh"]
