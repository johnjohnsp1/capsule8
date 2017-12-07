FROM alpine

# Install docker
RUN apk update && \
	apk upgrade && \
	apk --no-cache add docker

WORKDIR /functional

ENV GOTESTFLAGS=''

ADD . ./

CMD ./functional.test $GOTESTFLAGS
