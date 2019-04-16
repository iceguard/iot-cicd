package server

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog"
)

var (
	signalChan = make(chan os.Signal, 1)
)

const symlinkPath = "/tmp/iot-cicd"

// Server handles the http part of admission reviews / responses
type Server struct {
	server *http.Server
	mux    *http.ServeMux
	tls    tls
}

// flushWriter is used for streaming http responses
type flushWriter struct {
	f http.Flusher
	w io.Writer
}

// endpoints is a map that contains url to function mappings
type endpoints map[string]func(w http.ResponseWriter, r *http.Request)

type keyPair struct {
	tlsCertFile string
	tlsKeyFile  string
}

type tls struct {
	enabled bool
	keyPair
}

// New creates a new Server instance that will listen on the given address
func New(addr string) *Server {
	return &Server{
		server: &http.Server{
			Addr: addr,
		},
		mux: http.NewServeMux(),
		tls: tls{
			enabled: false,
		},
	}
}

// RegisterEndpoints can be used to register additional http endpoints and
// their corresponding functions
func (srv *Server) RegisterEndpoints(endpoints endpoints) {
	for url, function := range endpoints {
		klog.V(1).Infof("Registering additional endpoint %v", url)
		srv.mux.HandleFunc(url, function)
	}
}

// EnablePrometheus enables the prometheus endpoint on the given url
func (srv *Server) EnablePrometheus(url string) {
	klog.V(1).Infof("Registering prometheus handler on %v", url)
	srv.mux.Handle(url, promhttp.Handler())
}

// EnableTLS enables TLS encryption with the given certificate and key
func (srv *Server) EnableTLS(keypair keyPair) {
	klog.V(1).Infof("Enabling TLS")
	srv.tls.enabled = true
	srv.tls.keyPair = keypair
}

// Start starts the webhook server. This call is blocking and does not return
// until either Os.Shutdown signal or an error occurs
func (srv *Server) Start() error {
	srv.server.Handler = srv.mux
	errChan := srv.StartBackground()

	// listening for OS shutdown signal
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-signalChan:
		klog.Infof("Got shutdown signal")
		srv.Stop()
		return nil
	case err := <-errChan:
		return err
	}
}

// StartBackground starts the server in a goroutine and returns a channel that
// contains errors (if any)
func (srv *Server) StartBackground() chan error {
	klog.Infof("Starting Server...")
	// start webhook server in new routine
	errChan := make(chan error, 1)
	go func(errChan chan error) {
		var err error
		if srv.tls.enabled {
			err = srv.server.ListenAndServeTLS(srv.tls.tlsCertFile, srv.tls.tlsKeyFile)
		} else {
			err = srv.server.ListenAndServe()
		}

		if err != http.ErrServerClosed {
			errChan <- errors.Wrap(err, "error starting webserver")
		} else {
			errChan <- nil
		}
	}(errChan)
	return errChan
}

// Stop stops a running server
func (srv *Server) Stop() {
	klog.Infof("Stopping Server...")
	_ = srv.server.Shutdown(context.Background())
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return
}

// RegisterBuildHandler registers the endpoint for the build script and its
// arguments (if any)
func (srv *Server) RegisterBuildHandler(repoURL, endpoint, buildScript string, buildArgs, buildMasterArgs []string) {
	klog.Infof("Registering command \"%v %v\" on endpoint %v", buildScript, buildArgs, endpoint)
	srv.mux.HandleFunc(endpoint, buildHandler(repoURL, buildScript, buildArgs, buildMasterArgs))
}

func buildHandler(repoURL, buildScript string, buildArgs, buildMasterArgs []string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		errorResponse := func(errMsg string) {
			promStatusFailedBuild.Inc()
			klog.Errorf(errMsg)
			http.Error(w, errMsg, http.StatusFailedDependency)
		}

		klog.Infof("Got request to build")

		fw := flushWriter{w: w}
		if f, ok := w.(http.Flusher); ok {
			fw.f = f
		}

		// We need to create a buffer (and cannot give flushWriter),
		// as the prepareRepository method closes the Writer when
		// finished. We still need the flushWriter afterwards
		var buf bytes.Buffer
		buf.WriteTo(&fw)

		repo, err := setupRepo(repoURL, extractCommitID(r.URL), bufio.NewWriter(&buf))
		defer repo.cleanup()
		if err != nil {
			errorResponse("Error getting repository ready: " + err.Error())
			return
		}

		// The symlink servers as a static path to mount into the docker
		// container. If we do not have that, it would not be sufficient
		// just to stop and start the container again.
		err = os.Symlink(repo.path, symlinkPath)
		defer os.RemoveAll(symlinkPath)
		if err != nil {
			errorResponse("Error creating symlink: " + err.Error())
			return
		}

		var cmd *exec.Cmd
		if repo.branchName == "master" {
			cmd = exec.Command(filepath.Join(symlinkPath, buildScript), buildMasterArgs...)
		} else {
			cmd = exec.Command(filepath.Join(symlinkPath, buildScript), buildArgs...)
		}
		cmd.Stdout = &fw
		cmd.Stderr = &fw

		err = cmd.Run()
		if err != nil {
			errorResponse("Error executing command: " + err.Error())
			return
		}
		promStatusOK.Inc()
	}
}
