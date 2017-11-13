#!/bin/sh

PORT=${1:-80}

nc -l ${PORT} &
echo 'Hello, World!' | nc localhost ${PORT}
