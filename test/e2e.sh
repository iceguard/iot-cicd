#!/bin/sh

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

test_stuff() {
    # There is no way to check streaming output through curl
    # So please do that manually
    curl -s localhost:$PORT/build
}

echo "######################################################################"
echo "# As there is no possibility to test correct behaviour through curl, #"
echo "#       please check for yourself that the output is streaming       #"
echo "######################################################################"
echo ""
echo "> Press any button to start the test"
read -r
start_server
test_stuff
stop_server
