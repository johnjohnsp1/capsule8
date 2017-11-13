FROM ubuntu:16.04

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install build-essential && \
    apt-get clean

WORKDIR /exit_status

COPY status0.c status1.c status2.c ./

RUN make status0 status1 status2

# End with a command that returns status 0 so that running this container will
# not generate an error.
CMD sleep 1; ./status0;./status1; ./status2; pwd
