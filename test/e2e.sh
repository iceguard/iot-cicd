#!/bin/sh
set -e

PORT=49765
TEST_SCRIPT_NAME="script_test.sh"
SCRIPTPATH="$(dirname "$0")"

start_server() {
    "$SCRIPTPATH"/../iot-cicd -script "$SCRIPTPATH/$TEST_SCRIPT_NAME" -port "$PORT" &
    # Fix race condition
    sleep 1
}

stop_server() {
    pkill -TERM iot-cicd
}

test_build() {
    # There is no way to check streaming output through curl
    # so please do that manually
    curl -s localhost:$PORT/build
    statuscode="$(curl -s -o /dev/null localhost:$PORT/build -w "%{http_code}")"
    if [ "$statuscode" -ne 200 ]; then
        echo "Did not receive correct status code"
        echo "Got status code $statuscode"
        exit 1
    fi

    # Move away the script to trigger a failure
    mv "$SCRIPTPATH/$TEST_SCRIPT_NAME" "$SCRIPTPATH/$TEST_SCRIPT_NAME.bak"

    statuscode="$(curl -s -o /dev/null localhost:$PORT/build -w "%{http_code}")"
    if [ "$statuscode" -ne 424 ]; then
        echo "Did not receive correct status code for failure!"
        echo "Got status code $statuscode"
        exit 1
    fi

    # Move script back
    mv "$SCRIPTPATH/$TEST_SCRIPT_NAME.bak" "$SCRIPTPATH/$TEST_SCRIPT_NAME"
}

check_prometheus_endpoint() {
    successful_requests="$(curl -s localhost:$PORT/metrics | grep 'iot_cicd_requests_total{code="200"}' | awk '{print $2}')"
    failed_requests="$(curl -s localhost:$PORT/metrics | grep 'iot_cicd_requests_total{code="424"}' | awk '{print $2}')"
    if [ "$successful_requests" -ne 2 ] || [ "$failed_requests" -ne 1 ]; then
        echo "Prometheus did not log correctly:"
        echo "Successful requests expected: 2, actual: $successful_requests"
        echo "Failed requests expected: 1, actual: $failed_requests"
        exit 1
    fi
}

echo "######################################################################"
echo "# As there is no possibility to test correct behaviour through curl, #"
echo "#       please check for yourself that the output is streaming       #"
echo "######################################################################"
echo ""
echo ""
start_server
test_build
check_prometheus_endpoint
stop_server
echo ""
echo ""
echo "TEST SUCCESSFUL"
