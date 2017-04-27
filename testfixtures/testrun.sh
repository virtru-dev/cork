#!/bin/bash
CURR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
HOST_CACHE_DIR=${CURR_DIR}/.cork
HOST_WORK_DIR=${CURR_DIR}
WORK_DIR=${CURR_DIR}
CORK_DIR=${CURR_DIR}

mkdir -p $HOST_CACHE_DIR

$CURR_DIR/../builds/macos/cork-server --host-cache-dir=${HOST_CACHE_DIR} --host-work-dir=${HOST_WORK_DIR} --work-dir=${WORK_DIR} --dir=${CORK_DIR} 

echo $?

#CORK_SERVER_PID=$!

#sleep 3

#$CURR_DIR/../client_test/client-test

#kill $CORK_SERVER_PID
