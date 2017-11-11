FROM alpine

ARG vcsref
ARG version

LABEL org.label-schema.schema-version="1.0" \
      org.label-schema.name="Capsule8" \
      org.label-schema.description="Cloud-native telemetry" \
      org.label-schema.url="https://capsule8.io" \
      org.label-schema.vcs-url="https://github.com/capsule8/capsule8" \
      org.label-schema.vcs-ref="${vcsref}" \
      org.label-schema.version="${version}"

# Exposing the API over a TCP port is the default when running in a container
ENV CAPSULE8_SENSOR_SERVER_ADDR=:8484

COPY bin/sensor sensor
CMD ["/sensor"]

# HTTP Monitoring
EXPOSE 8080

# API gRPC
EXPOSE 8484

# For persistent stateful data (e.g. flight recorder event store)
VOLUME /var/lib/capsule8
