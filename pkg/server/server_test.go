package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/src-d/go-git.v4"
)

func TestWebhookServerSIGINT(t *testing.T) {
	srv := New("localhost:51467")
	srv.EnablePrometheus("/metrics")
	// as srv.Start() blocks, run a go routine to send the stop signal
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()

	signalChan <- syscall.SIGINT

	assert.Nil(t, <-errChan)
}

func TestWebhookTLS(t *testing.T) {
	srv := New("localhost:51467")
	srv.EnableTLS(keyPair{
		tlsCertFile: "doesnotexist.crt",
		tlsKeyFile:  "doesnotexist.key",
	})

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start()
	}()
	assert.NotNil(t, <-errChan)
}

func TestBuildHandler(t *testing.T) {
	// Open the git repo that contains this program to
	// extract a valid commit id
	r, err := git.PlainOpen("../..")
	if err != nil {
		t.Fatalf("Error opening git repo: %v", err)
	}
	// extract the reference from the current head
	ref, err := r.Head()
	if err != nil {
		t.Fatalf("Error getting Head reference: %v", err)
	}

	handlertests := []struct {
		in         []string
		out        string
		statusCode int
	}{
		{[]string{"test/script_test.sh"}, "Streaming 1!\nStreaming 2!\nStreaming 3!\n", 200},
		{[]string{"test/nonexistent.sh"}, "test/nonexistent.sh: no such file or directory\n", 424},
	}

	for i, tt := range handlertests {
		t.Run(fmt.Sprintf("cmd Handler Test %v", i), func(t *testing.T) {
			handlerFunc := buildHandler("https://github.com/iceguard/iot-cicd.git", tt.in[0], tt.in[1:]...)
			req, err := http.NewRequest("GET", "/build/"+ref.Hash().String(), nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlerFunc)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.out)
		})
	}
}
