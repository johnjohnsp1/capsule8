# Load / Functional Tests

Any tests that require us to run the entire service ecosystem. Each layer is a horizontally scalable service.
  * Run services first via `docker-compose -f .buildkite/reactive8-compose.yml up`
  * Run functional tests via `go test ./tests/functional`

        APISERVER
            |
            |
            |
       STAN / NATS
         /  |  \
       /    |    \
     /      |      \
SENSOR    SENSOR   SENSOR   *Note*: `.buildkite/reactive8-compose.yml` only runs a single node sensor


Additionally, `SENSOR` is actually a `NODE SENSOR` which listens for subscriptions and creates/destroys various event generating `SENSOR`s.
