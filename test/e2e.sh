#!/bin/bash

PORT=49765
TEST_SCRIPT_NAME="script_test.sh"
SCRIPTPATH="$(dirname "$0")"

start_server() {
    "$SCRIPTPATH"/../iot-cicd -script "test/$TEST_SCRIPT_NAME" -port "$PORT" -repo-url "https://github.com/iceguard/iot-cicd.git" -v 1 &
    # Fix race condition
    sleep 1
}

stop_server() {
    pkill -TERM iot-cicd
}

test_build() {
    # There is no way to check streaming output through curl
    # so please do that manually
    curl -s localhost:$PORT/build/
    statuscode="$(curl -s -o /dev/null localhost:$PORT/build/ -w "%{http_code}")"
    if [ "$statuscode" -ne 200 ]; then
        echo "Did not receive correct status code"
        error ${LINENO} "Got status code $statuscode, expected 200"
        false
    fi
    successful_requests="$(curl -s localhost:$PORT/metrics | grep 'iot_cicd_requests_total{code="200"}' | awk '{print $2}')"
    if [ "$successful_requests" -ne 2 ]; then
        echo "Prometheus did not log correctly:"
        error ${LINENO} "Successful requests expected: 2, actual: $successful_requests"
        false
    fi


    stop_server
    TEST_SCRIPT_NAME="nonexisting_script.sh"
    start_server

    statuscode="$(curl -s -o /dev/null localhost:$PORT/build/ -w "%{http_code}")"
    if [ "$statuscode" -ne 424 ]; then
        echo "Did not receive correct status code for failure!"
        error ${LINENO} "Got status code $statuscode, expected 424"
        false
    fi
    failed_requests="$(curl -s localhost:$PORT/metrics | grep 'iot_cicd_requests_total{code="424"}' | awk '{print $2}')"
    if [ "$failed_requests" -ne 1 ]; then
        echo "Prometheus did not log correctly:"
        error ${LINENO} "Failed requests expected: 1, actual: $failed_requests"

    fi
}


trap stop_server 0
error() {
  local parent_lineno="$1"
  local message="$2"
  local code="${3:-1}"
  if [[ -n "$message" ]] ; then
    echo "Error on or near line ${parent_lineno}: ${message}; exiting with status ${code}"
    echo "######################################################################"
    echo "#                               TEST FAILED                          #"
    echo "######################################################################"
  else
    echo "Error on or near line ${parent_lineno}; exiting with status ${code}"
    echo "######################################################################"
    echo "#                               TEST FAILED                          #"
    echo "######################################################################"
  fi
  exit "${code}"
}
trap 'error ${LINENO}' ERR

echo "######################################################################"
echo "# As there is no possibility to test correct behaviour through curl, #"
echo "#       please check for yourself that the output is streaming       #"
echo "######################################################################"
echo ""
echo ""
start_server
test_build
echo "######################################################################"
echo "#                            TEST SUCCESSFUL                         #"
echo "######################################################################"
