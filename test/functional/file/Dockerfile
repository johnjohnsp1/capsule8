FROM alpine

RUN apk upgrade && \
	apk add --no-cache alpine-sdk

WORKDIR /
COPY testdata/filedata.txt init.c open.c /

RUN make init open && ./init < filedata.txt

CMD ./open < filedata.txt
