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

## Using

Once the webserver is started, it is ready to accept incoming connections
on `/build`.
When doing a request like this:

```
> curl http://iot-cicd/build/[commit-id]
```

the server is going to check out the git repository on the specified
commit id and build the software with this.
If no commit id is given (e.g. a plain call on `http://iot-cicd/build`),
master will be used to build.

## Monitoring

There is a built-in monitoring endpoint reachable on `/metrics`. It is
a prometheus endpoint with some custom metrics.
