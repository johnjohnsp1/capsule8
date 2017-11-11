FROM golang:1.9

#
# We need to add this for clang to enable msan test
#
RUN apt-get update
RUN apt-get -y install git make clang

# Install go packages
RUN go get -u github.com/golang/lint/golint

# Install docker
RUN curl -fsSL get.docker.com | bash