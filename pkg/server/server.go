package server

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/src-d/go-git.v4"
	gitplumbing "gopkg.in/src-d/go-git.v4/plumbing"

	"k8s.io/klog"
)

var (
	signalChan = make(chan os.Signal, 1)
)

// Server handles the http part of admission reviews / responses
type Server struct {
	server *http.Server
	mux    *http.ServeMux
	tls    tls
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

type buildContext struct {
	commitID string
	repoPath string
	repoURL  string
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
func (srv *Server) RegisterBuildHandler(repoURL, endpoint, buildScript string, args ...string) {
	klog.Infof("Registering command \"%v %v\" on endpoint %v", buildScript, args, endpoint)
	srv.mux.HandleFunc(endpoint, buildHandler(repoURL, buildScript, args...))
}

func buildHandler(repoURL, buildScript string, args ...string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		klog.Infof("Got request to build")

		commitID := extractCommitID(r.URL)

		fw := flushWriter{w: w}
		if f, ok := w.(http.Flusher); ok {
			fw.f = f
		}

		// We need to create a buffer (and cannot give flushWriter),
		// as the prepareRepository method closes the Writer when
		// finished. We still need the flushWriter afterwards
		var buf bytes.Buffer
		buf.WriteTo(&fw)
		repoPath, err := prepareRepository(repoURL, commitID, &buf)
		defer os.RemoveAll(repoPath)
		if err != nil {
			promStatusFailedBuild.Inc()
			klog.Errorf("Error cloning repository: %v", err)
			http.Error(w, err.Error(), http.StatusFailedDependency)
			return
		}

		cmd := exec.Command(filepath.Join(repoPath, buildScript), args...)
		cmd.Stdout = &fw
		cmd.Stderr = &fw
		err = cmd.Run()
		if err != nil {
			promStatusFailedBuild.Inc()
			klog.Errorf("Error executing command: %v", err)
			http.Error(w, err.Error(), http.StatusFailedDependency)
			return
		}
		promStatusOK.Inc()
	}
}

// extractCommitID takes the URL and extracts the commit id
func extractCommitID(url *url.URL) string {
	var commitID string
	if url.Path[1:] != "build" {
		commitID = strings.Replace(url.Path[1:], "build/", "", 1)
	}

	return commitID
}

// prepareRepository checks out the given git repository and commit from
// the build context URL into the build context path and writes the clone
// output to the given writer.
// If no commitID is given, the default branch is checked out
func prepareRepository(repoURL, commitID string, w io.Writer) (repoPath string, err error) {
	repoPath, err = ioutil.TempDir("", "iot-cicd")
	if err != nil {
		return "", err
	}

	klog.Infof("Cloning repo %v to %v", repoURL, repoPath)
	r, err := git.PlainClone(repoPath, false, &git.CloneOptions{
		URL:      repoURL,
		Progress: w,
	})
	if err != nil {
		return "", errors.Wrapf(err, "Error cloning module %v", repoURL)
	}

	if commitID == "" {
		return repoPath, nil
	}

	klog.V(1).Infof("checking out commit id %v to %v", commitID, repoPath)
	wt, err := r.Worktree()
	if err != nil {
		return "", errors.Wrapf(err, "Error extracting worktree")
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Hash: gitplumbing.NewHash(commitID),
	})
	if err != nil {
		return "", errors.Wrapf(err, "Error checking out commid id %v", commitID)
	}

	return repoPath, nil
}
