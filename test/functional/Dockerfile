FROM alpine

# Install docker
RUN apk update && \
	apk upgrade && \
	apk --no-cache add docker openrc && \
	rc-update add docker boot

WORKDIR /functional

ADD . ./

CMD ./functional.test
