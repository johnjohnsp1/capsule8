#!/bin/sh

PORT=${1:-80}

(sleep 1; echo 'Hello, TCP4!') | timeout 5 nc -4 localhost ${PORT} &
timeout 2 nc -4l ${PORT} | grep TCP4

(sleep 1; echo 'Hello, TCP6!') | timeout 5 nc -6 localhost ${PORT} &
timeout 2 nc -6l ${PORT} | grep TCP6

(sleep 1; echo 'Hello, UDP4!') | timeout 5 nc -4u localhost ${PORT} &
timeout 2 nc -4lu ${PORT} | grep UDP4

(sleep 1; echo 'Hello, UDP6!') | timeout 5 nc -6u localhost ${PORT} &
timeout 2 nc -6lu ${PORT} | grep UDP6
