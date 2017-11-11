FROM alpine

RUN apk -U add docker

WORKDIR /functional
COPY inner/Dockerfile /functional/

CMD docker build -t c8-sensor-container-functest . && \
	docker run --rm c8-sensor-container-functest
