package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
)

const (
	errDecodingAdmissionReview = "error decoding admission review"
	errEmptyHTTPBody           = "empty body"
	errEncodingAdmissionReview = "error encoding admission review"
	errInvalidHTTPContentType  = "invalid Content-Type, expect `application/json`"
	errListenServeWebhook      = "failed to listen and serve webhook server"
	errWritingHTTPBody         = "error writing HTTP body"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
	signalChan    = make(chan os.Signal, 1)
)

// Server handles the http part of admission reviews / responses
type Server struct {
	server    *http.Server
	mux       *http.ServeMux
	endpoints endpoints
	tls       tls
}

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

// New is a constructor
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

// RegisterEndpoints can be used to register additional http endpoints and the corresponding functions
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

// Start starts the webhook server. This call is blocking and does not return until
// either Os.Shutdown signal or an error occurs
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

// StartBackground starts the server in a goroutine and returns a channel that contains
// errors (if any)
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
			errChan <- errors.Wrap(err, errListenServeWebhook)
		} else {
			errChan <- nil
		}
	}(errChan)
	return errChan
}

// Stop stops a running server
func (srv *Server) Stop() {
	klog.Infof("Stopping Server...")
	srv.server.Shutdown(context.Background())
}

func (fw *flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if fw.f != nil {
		fw.f.Flush()
	}
	return
}

// RegisterBuildHandler registers the endpoint for the build script and its arguments (if any)
func (srv *Server) RegisterBuildHandler(endpoint, buildScript string, args ...string) {
	klog.Infof("Registering command \"%v %v\" on endpoint %v", buildScript, args, endpoint)
	srv.mux.HandleFunc(endpoint, buildHandler(buildScript, args...))
}

func buildHandler(buildScript string, args ...string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("Got request to exec command")
		fw := flushWriter{w: w}
		if f, ok := w.(http.Flusher); ok {
			fw.f = f
		}
		cmd := exec.Command(buildScript, args...)
		cmd.Stdout = &fw
		cmd.Stderr = &fw
		err := cmd.Run()
		if err != nil {
			klog.Errorf("Error executing command: %v", err)
		}
	}
}
