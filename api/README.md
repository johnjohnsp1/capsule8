# Capsule8 APIs and Common Configuration Definitions

This directory defines component-level APIs and common configuration
formats for the Capsule8 platform. These definitions also serve as the
ground truth of the shared vocabulary used by Capsule8.

This directory will eventually be moved to an independent
repository so that external capsulators may clone/submodule it in as
necessary.

These definitions are specified using the
[protobuf](https://github.com/google/protobuf) syntax and follow
Google's [API Design Guide](https://cloud.google.com/apis/design/).
For useful examples, see the public
[Google APIs](https://github.com/googleapis/googleapis).

# Packages

## Event

The Event API is the externally-facing interface to both internally
and externally developed capsulators. It is the only interface that is
versioned as well as the only API that supports HTTP+JSON via the
[gRPC REST Gateway](https://github.com/grpc-ecosystem/grpc-gateway)

## Backplane

The Backplane provides distributed real-time messaging and persistent
configuration for internal components.

## Sensor

The Sensor runs on each node, collecting and transmitting events
that match the configured Selectors for active subscriptions.

## Recorder

The Recorder runs on each node maintaining persistent
subscriptions to the local Sensor for logging into a persistent
circular datastore.
