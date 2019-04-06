package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
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
	handlertests := []struct {
		in  []string
		out string
	}{
		{[]string{"echo", "-n", "hello", "world"}, "hello world"},
		{[]string{"sh", "-c", "echo $USER"}, os.Getenv("USER") + "\n"},
	}

	for i, tt := range handlertests {
		t.Run(fmt.Sprintf("cmd Handler Test %v", i), func(t *testing.T) {
			handlerFunc := buildHandler(tt.in[0], tt.in[1:]...)
			req, err := http.NewRequest("GET", "/build", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(handlerFunc)

			handler.ServeHTTP(rr, req)

			// Check the status code is what we expect.
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v",
					status, http.StatusOK)
			}

			// Check the response body is what we expect.
			expected := tt.out
			if rr.Body.String() != expected {
				t.Errorf("handler returned unexpected body: got %v want %v",
					rr.Body.String(), expected)
			}
		})
	}
}
