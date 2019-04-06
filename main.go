package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/iceguard/iot-cicd/pkg/server"
	"k8s.io/klog"
)

func main() {
	klog.InitFlags(nil)
	defer klog.Flush()
	serverAddr := flag.String("address", "127.0.0.1", "address (interface) to listen on")
	serverPort := flag.Int("port", 8080, "port to listen on for requests")
	buildScript := flag.String("script", "./build.sh", "path to the build script")
	buildArg := flag.String("args", "", "comma-separated list of arguments given to the build script")
	flag.Parse()

	buildArgs := strings.Split(*buildArg, ",")

	srv := server.New(fmt.Sprintf("%v:%v", *serverAddr, *serverPort))
	srv.RegisterBuildHandler("/build", *buildScript, buildArgs...)

	klog.Infof("Starting to serve on %v:%v\n", *serverAddr, *serverPort)
	err := srv.Start()
	if err != nil {
		klog.Errorf("Starting Server failed: %v", err)
		os.Exit(1)
	}
}
