FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

WORKDIR /
COPY testdata/filedata.txt init.c open.c /

RUN make init open && ./init < filedata.txt

CMD ./open < filedata.txt
