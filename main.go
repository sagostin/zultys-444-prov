package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	listenAddr = flag.String("listen", ":444", "Address to listen on")
	certFile   = flag.String("cert", "server.crt", "Path to SSL certificate")
	keyFile    = flag.String("key", "server.key", "Path to SSL key")
	insecure   = flag.Bool("insecure", false, "Skip upstream TLS verification")
	oldPath    = "/httpsphone2/"
	newPath    = "/httpsphone/"
)

type ProxyHandler struct {
	Client *http.Client
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// 1. Determine Target Host from Request
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	targetHost := fmt.Sprintf("%s:443", host)

	// 2. Construct Upstream URL
	u := *r.URL
	u.Scheme = "https"
	u.Host = targetHost

	// Simple Rewrite Logic
	// Replaces "/httpsphone2/" with "/httpsphone/" at the start of the string.
	// This preserves the "hidden path" (e.g. D12C54) and any specific file requested (e.g. 000bea89e6c9.cfg).
	if strings.HasPrefix(r.URL.Path, oldPath) {
		u.Path = strings.Replace(r.URL.Path, oldPath, newPath, 1)
	} else {
		u.Path = r.URL.Path
	}
	u.RawQuery = r.URL.RawQuery

	log.Printf("Proxying %s %s -> %s", r.Method, r.URL.Path, u.String())

	// 3. Create Upstream Request
	req, err := http.NewRequest(r.Method, u.String(), r.Body)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		log.Printf("Error creating request: %v", err)
		return
	}

	// 4. Copy Headers
	for k, v := range r.Header {
		if isHopByHop(k) {
			continue
		}
		req.Header[k] = v
	}
	req.Host = targetHost

	// 5. Send Request
	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, "Upstream error", http.StatusBadGateway)
		log.Printf("Upstream error: %v", err)
		return
	}
	defer resp.Body.Close()

	// 6. Copy Response Headers
	for k, v := range resp.Header {
		if isHopByHop(k) {
			continue
		}
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	// 7. Copy Response Body
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response: %v", err)
	}

	log.Printf("Completed %d %s in %v (%d bytes)", resp.StatusCode, u.Path, time.Since(start), n)
}

func main() {
	flag.Parse()

	// Configure upstream client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecure},
		Proxy:           http.ProxyFromEnvironment,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	h := &ProxyHandler{Client: client}
	http.Handle("/", h)

	log.Printf("Starting Zultys Proxy on %s (Dynamic Target: host:443)", *listenAddr)
	err := http.ListenAndServeTLS(*listenAddr, *certFile, *keyFile, nil)
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// isHopByHop checks if a header is a hop-by-hop header that should not be forwarded
func isHopByHop(header string) bool {
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}
	for _, h := range hopHeaders {
		if strings.EqualFold(header, h) {
			return true
		}
	}
	return false
}
