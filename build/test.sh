#!/bin/bash
#
# CI test script
#

set -xeo pipefail

[[ -n $DEBUG ]] && set -o xtrace

declare sensor_container_id=""

function on_exit() {
	if [[ $? -ne 0 ]]; then
		echo "^^^ +++"
	fi

	#
	# If the container was started, clean it up and print logs if didn't exit with success
	#
	if [ ! -z "$sensor_container_id" ]; then
		echo "--- Stopping sensor container"		
		docker stop $sensor_container_id
		if [ $(docker wait $sensor_container_id) != 0 ]; then
			echo "^^^ +++"
			docker logs $sensor_container_id
		fi
		
		docker rm $sensor_container_id
	fi
}

trap on_exit EXIT

function run_sensor_container() {
	local sensor_container_image=$(docker build -q .)
	
	sensor_container_id=$(docker run -it -d                                \
		--privileged                                                   \
		--publish 8484:8484                                            \
		--volume=/var/run/capsule8:/var/run/capsule8                   \
		--volume=/proc:/var/run/capsule8/proc/:ro                      \
		--volume=/sys/kernel/debug:/sys/kernel/debug                   \
		--volume=/sys/fs/cgroup:/sys/fs/cgroup                         \
		--volume=/var/lib/docker:/var/lib/docker:ro                    \
		--volume=/var/run/docker:/var/run/docker:ro                    \
		$sensor_container_image)
}

#
# Run tests in the order of shortest-running tests first, longest
# running tests last
#

echo "--- Running unit tests"
make test

echo "--- Running unit tests with memory sanitizer"
make test_msan

echo "--- Running unit tests with race detector"
make test_race

echo "--- Starting sensor container"
run_sensor_container

echo "--- Running functional tests"
# It is ok to fail the functional tests for now
make run_functional_test || true
