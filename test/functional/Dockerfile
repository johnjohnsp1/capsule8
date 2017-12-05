FROM alpine

# Install docker
RUN apk update && \
	apk upgrade && \
	apk --no-cache add docker openrc && \
	rc-update add docker boot

WORKDIR /functional

ENV GOTESTFLAGS=''

ADD . ./

CMD ./functional.test $GOTESTFLAGS
