FROM alpine

# Install docker
RUN apk update && \
	apk upgrade && \
	apk --no-cache add docker

WORKDIR /functional

# ENV CAPSULE8_API_SERVER='localhost:8484'
ENV GOTESTFLAGS=''

ADD . ./

CMD ./functional.test $GOTESTFLAGS
