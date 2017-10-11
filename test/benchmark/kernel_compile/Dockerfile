FROM ubuntu:16.04
ARG kernel=4.1

RUN apt-get update && \
    apt-get -y upgrade && \
    apt-get -y install \
	    bc \
	    binutils \
	    clang \
	    gcc \
	    libc-dev \
	    libelf-dev \
	    libssl-dev \
	    llvm \
	    make \
	    tar \
	    time \
	    wget \
	    xz-utils && \
    apt-get clean

RUN cd / && \
    wget https://www.kernel.org/pub/linux/kernel/v4.x/linux-${kernel}.tar.xz

WORKDIR /usr/src

#
# Do all of the intensive work in the CMD
#
CMD /usr/bin/time bash -c "tar -x -J -f /linux-*.tar.xz && cd linux-* && make defconfig && make -j$(expr 2 \* $(grep -c '^processor' /proc/cpuinfo)) bzImage && make -j$(expr 2 \* $(grep -c '^processor' /proc/cpuinfo)) clean" >/dev/null
