# IoT CI/CD

This repository contains a program that will start a Webserver, on call to
`/build` exec the given command and stream the output back to the callee

## Building

To build this software, you need the Golang toolchain installed.
Then, just call

```
go build .
```

to build the software. It will create a binary called `iot-cicd`

## Testing

The unit tests can be triggered through calling

```
go test ./...
```

Additionally, if you want more output, call

```
go test ./... -v -cover
```

There is also a folder called `test` that contain the `e2e.sh` script.
This script starts the program and curl the endpoint.
As curl does not provide the ability to stream output, you need
to check yourself if the output is streaming.
To execute the e2e script, call

```
./test/e2e.sh
```

## Starting the Webserver

All the configuration is done through command line arguments.
See `iot-cicd --help` for more information
