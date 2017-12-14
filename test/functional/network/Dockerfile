FROM golang:alpine

WORKDIR /go/src/app
COPY . .

RUN apk add -U git
RUN go-wrapper download   # "go get -d -v ./..."
RUN go-wrapper install    # "go install -v ./..."

CMD ["go-wrapper", "run"] # ["app"]
