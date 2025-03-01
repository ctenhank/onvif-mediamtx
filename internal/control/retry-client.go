package control

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"time"
)

const RetryCount = 3

type retryableTransport struct {
	transport http.RoundTripper
}

func backoff(retries int) time.Duration {
	return time.Duration(math.Pow(2, float64(retries))) * time.Second
}

func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		fmt.Println("shouldRetry: yes", err, resp.Request.URL)
		return true
	}

	if resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusInternalServerError {
		fmt.Println("shouldRetry: yes", resp.StatusCode, resp.Request.URL)
		return true
	}

	fmt.Println("shouldRetry: no", resp.StatusCode, resp.Request.URL)

	return false
}

func drainBody(resp *http.Response) {
	if resp.Body != nil {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Send the request
	resp, err := t.transport.RoundTrip(req)

	// Retry logic
	retries := 0
	for shouldRetry(err, resp) && retries < RetryCount {
		// Wait for the specified backoff period
		time.Sleep(backoff(retries))

		// We're going to retry, consume any response to reuse the connection.
		drainBody(resp)

		// Clone the request body again
		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Retry the request
		resp, err = t.transport.RoundTrip(req)

		retries++
	}

	// Return the response
	return resp, err
}

func NewRetryableClient() *http.Client {
	transport := &retryableTransport{
		transport: &http.Transport{},
	}

	return &http.Client{
		Transport: transport,
	}
}
