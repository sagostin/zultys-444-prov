package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockTransport allows us to capture the outgoing request
type MockTransport struct {
	RoundTripFunc func(*http.Request) (*http.Response, error)
}

func (m *MockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.RoundTripFunc(req)
}

func TestProxyHandler(t *testing.T) {
	// 1. Setup Mock Client
	var capturedReq *http.Request
	mockClient := &http.Client{
		Transport: &MockTransport{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				capturedReq = req
				// Return a fake response
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString("OK")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	h := &ProxyHandler{Client: mockClient}

	// 2. Create Test Request
	// Simulate client connecting to port 444
	req := httptest.NewRequest("GET", "https://zultys.topsoffice.ca:444/httpsphone2/D12C54", nil)
	// httptest.NewRequest sets Host based on URL, or we can override
	req.Host = "zultys.topsoffice.ca:444"

	w := httptest.NewRecorder()

	// 3. Serve
	h.ServeHTTP(w, req)

	// 4. Verify Upstream Request
	if capturedReq == nil {
		t.Fatal("Upstream request was not made")
	}

	// Check Target URL
	expectedHost := "zultys.topsoffice.ca:443"
	if capturedReq.URL.Host != expectedHost {
		t.Errorf("Expected upstream host %s, got %s", expectedHost, capturedReq.URL.Host)
	}

	// Check Path Rewrite
	expectedPath := "/httpsphone/D12C54"
	if capturedReq.URL.Path != expectedPath {
		t.Errorf("Expected upstream path %s, got %s", expectedPath, capturedReq.URL.Path)
	}

	// 5. Verify Response
	if w.Result().StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", w.Result().StatusCode)
	}
}

func TestProxyHandler_PreservesFilename(t *testing.T) {
	// Setup Mock Client
	var capturedReq *http.Request
	mockClient := &http.Client{
		Transport: &MockTransport{
			RoundTripFunc: func(req *http.Request) (*http.Response, error) {
				capturedReq = req
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString("CONFIG DATA")),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	h := &ProxyHandler{Client: mockClient}

	// Test with a full file path (Hidden Path + MAC.cfg)
	// Input: /httpsphone2/D12C54/000bea89e6c9.cfg
	req := httptest.NewRequest("GET", "https://zultys.topsoffice.ca:444/httpsphone2/D12C54/000bea89e6c9.cfg", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	// Verify Rewrite
	// Expected: /httpsphone/D12C54/000bea89e6c9.cfg (2 removed, rest preserved)
	expectedPath := "/httpsphone/D12C54/000bea89e6c9.cfg"
	if capturedReq.URL.Path != expectedPath {
		t.Errorf("Path mismatch.\nWant: %s\nGot:  %s", expectedPath, capturedReq.URL.Path)
	}
}
