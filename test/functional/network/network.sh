#!/bin/sh

PORT=${1:-80}

nc -l ${PORT} &
sleep 1
echo 'Hello, World!' | nc localhost ${PORT}
