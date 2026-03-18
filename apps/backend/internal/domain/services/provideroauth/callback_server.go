package provideroauth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const successHTML = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"/><title>Authentication successful</title></head>
<body><p>Authentication successful. Return to your terminal to continue.</p></body>
</html>`

// callbackResult holds the code and state received from the OAuth callback.
type callbackResult struct {
	Code  string
	State string
}

// callbackServer runs a temporary local HTTP server to receive OAuth callbacks.
type callbackServer struct {
	server   *http.Server
	listener net.Listener
	port     int
	path     string

	mu        sync.Mutex
	result    *callbackResult
	cancelled bool
	done      chan struct{}
	received  bool
}

// startCallbackServer creates and starts a local HTTP server on the given port
// that listens for OAuth callbacks at the given path. It validates the state
// parameter against expectedState.
func startCallbackServer(port int, path string, expectedState string) (*callbackServer, error) {
	cs := &callbackServer{
		port: port,
		path: path,
		done: make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		defer cs.mu.Unlock()

		code := r.URL.Query().Get("code")
		state := r.URL.Query().Get("state")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>Error: %s</p></body></html>", errParam)
			return
		}

		if code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body><h1>Authentication Failed</h1><p>Missing authorization code.</p></body></html>")
			return
		}

		if expectedState != "" && state != expectedState {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<html><body><h1>Authentication Failed</h1><p>State mismatch.</p></body></html>")
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, successHTML)
		cs.result = &callbackResult{Code: code, State: state}
		cs.received = true
		select {
		case cs.done <- struct{}{}:
		default:
		}
	})

	var err error
	cs.listener, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, fmt.Errorf("bind callback server on port %d: %w", port, err)
	}

	cs.server = &http.Server{Handler: mux}
	go cs.server.Serve(cs.listener) //nolint:errcheck

	return cs, nil
}

// waitForCode blocks until a callback is received, the context is cancelled,
// or the timeout expires. Returns nil if cancelled or timed out.
func (cs *callbackServer) waitForCode(ctx context.Context, timeout time.Duration) *callbackResult {
	// Check if result already arrived before we start waiting.
	cs.mu.Lock()
	if cs.received {
		r := cs.result
		cs.mu.Unlock()
		return r
	}
	cs.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case <-cs.done:
		cs.mu.Lock()
		defer cs.mu.Unlock()
		return cs.result
	case <-ctx.Done():
		return nil
	}
}

// close shuts down the callback server.
func (cs *callbackServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cs.server.Shutdown(ctx) //nolint:errcheck
}

// redirectURI returns the full redirect URI for this callback server.
func (cs *callbackServer) redirectURI() string {
	return fmt.Sprintf("http://localhost:%d%s", cs.port, cs.path)
}

// parseAuthorizationInput attempts to extract code and state from user input,
// which may be a full redirect URL, query string, or raw code.
func parseAuthorizationInput(input string) (code, state string) {
	if input == "" {
		return "", ""
	}

	// Try parsing as URL
	if u, err := url.Parse(input); err == nil && u.Scheme != "" {
		return u.Query().Get("code"), u.Query().Get("state")
	}

	// Try as query string
	if vals, err := url.ParseQuery(input); err == nil && vals.Get("code") != "" {
		return vals.Get("code"), vals.Get("state")
	}

	// Treat as raw code
	return input, ""
}
