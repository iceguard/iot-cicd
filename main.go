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
	// All those flags 🏁 🏴 🏳️
	klog.InitFlags(nil)
	defer klog.Flush()
	serverAddr := flag.String("address", "127.0.0.1", "address (interface) to listen on")
	serverPort := flag.Int("port", 8080, "port to listen on for requests")
	buildScript := flag.String("script", "Device/build.sh", "relative path to the build script")
	buildMasterArg := flag.String("master-args", "", "comma-separated list of arguments given to the build script when building on master")
	buildArg := flag.String("args", "", "comma-separated list of arguments given to the build script")
	repoURL := flag.String("repo-url", "https://github.com/iceguard/mxchip", "Git URL to clone for build process")
	flag.Parse()

	buildArgs := strings.Split(*buildArg, ",")
	buildMasterArgs := strings.Split(*buildMasterArg, ",")

	// Set up the server
	srv := server.New(fmt.Sprintf("%v:%v", *serverAddr, *serverPort))
	srv.RegisterBuildHandler(*repoURL, "/build/", *buildScript, buildArgs, buildMasterArgs)
	srv.EnablePrometheus("/metrics")

	klog.Infof("Starting to serve on %v:%v\n", *serverAddr, *serverPort)
	err := srv.Start()
	if err != nil {
		klog.Errorf("Starting Server failed: %v", err)
		os.Exit(1)
	}
}
