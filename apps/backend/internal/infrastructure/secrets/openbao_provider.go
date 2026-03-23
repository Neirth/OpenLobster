package secrets

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
)

const (
	// openbaoTimeout is the per-attempt HTTP deadline for every OpenBao call.
	// Kept short so individual attempts fail fast; retries handle transient issues.
	openbaoTimeout = 5 * time.Second

	// Retry knobs for Get: exponential backoff with jitter.
	// Total worst-case wait before giving up: ~14 s (0 + 2 + 4 + 8 s of sleep).
	openbaoMaxRetries    = 3
	openbaoRetryBaseWait = 2 * time.Second

	openbaoValueKey = "value"
)

// OpenBAOProvider implements SecretsProvider using the HashiCorp Vault API client.
// OpenBao is API-compatible with Vault, so the official SDK works against both.
// It supports both KV v1 and KV v2 secrets engines: KV v2 is tried first (common default
// in OpenBao); if the server returns 404, operations fall back to KV v1 paths.
type OpenBAOProvider struct {
	client *vault.Client
	mount  string
}

// NewOpenBAOProvider creates a secrets provider that talks to OpenBao (or Vault) at address
// using the given token. Mount is the secrets engine path (e.g. "secret"). Both KV v1 and
// KV v2 engines are supported; KV v2 is used when the path exists.
func NewOpenBAOProvider(address, token, mount string) (*OpenBAOProvider, error) {
	if mount == "" {
		mount = "secret"
	}
	mount = strings.TrimSuffix(mount, "/")
	address = strings.TrimSuffix(address, "/")

	cfg := vault.DefaultConfig()
	cfg.Address = address
	cfg.HttpClient.Timeout = openbaoTimeout
	// Disable HTTP keep-alives for test environments where httptest.Server
	// Close() can block waiting for active connections. Disabling keep-alive
	// makes individual requests close connections promptly.
	if cfg.HttpClient.Transport == nil {
		cfg.HttpClient.Transport = &http.Transport{DisableKeepAlives: true}
	} else {
		if tr, ok := cfg.HttpClient.Transport.(*http.Transport); ok {
			tr.DisableKeepAlives = true
		}
	}

	client, err := vault.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	client.SetToken(token)

	return &OpenBAOProvider{
		client: client,
		mount:  mount,
	}, nil
}

// pathV1 returns the path for KV v1 (e.g. secret/mcp/remote/notion/token).
func (p *OpenBAOProvider) pathV1(key string) string {
	if key == "" {
		return p.mount + "/"
	}
	return p.mount + "/" + key
}

// pathV2 returns the path for KV v2 read/write/delete (e.g. secret/data/mcp/remote/notion/token).
func (p *OpenBAOProvider) pathV2(key string) string {
	if key == "" {
		return p.mount + "/data/"
	}
	return p.mount + "/data/" + key
}

// pathV2List returns the path for KV v2 list (e.g. secret/metadata/mcp/remote).
func (p *OpenBAOProvider) pathV2List(prefix string) string {
	if prefix == "" {
		return p.mount + "/metadata/"
	}
	return p.mount + "/metadata/" + strings.TrimSuffix(prefix, "/")
}

// isNotFound returns true if the error is an HTTP 404 from Vault/OpenBao.
func isNotFound(err error) bool {
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

// formatOpenbaoError normalizes errors returned by the Vault/OpenBao client
// so log messages are more useful (HTTP status codes and common causes).
func formatOpenbaoError(op, key string, err error) error {
	if err == nil {
		return nil
	}

	// Context cancellations: indicate timeout or process shutdown.
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("openbao %s %q: operation canceled (context canceled: check connectivity/network or timeouts)", op, key)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("openbao %s %q: deadline exceeded (OpenBao did not respond in time)", op, key)
	}

	// HTTP errors from the Vault/OpenBao server.
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		joined := strings.Join(respErr.Errors, "; ")
		// Very common case: Vault/OpenBao sealed.
		if respErr.StatusCode == 503 && strings.Contains(strings.ToLower(joined), "sealed") {
			return fmt.Errorf("openbao %s %q: sealed backend (HTTP %d: %s)", op, key, respErr.StatusCode, joined)
		}
		return fmt.Errorf("openbao %s %q: HTTP error from backend (status=%d): %s", op, key, respErr.StatusCode, joined)
	}

	// Generic fallback preserving the original error.
	return fmt.Errorf("openbao %s %q: %w", op, key, err)
}

// isTransient returns true for errors that are worth retrying: timeouts and
// context deadline/cancellation caused by network hiccups or cold-start latency.
// HTTP 4xx/5xx application errors are NOT transient and should not be retried.
func isTransient(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	// Vault SDK wraps net/url errors for connection-refused / EOF.
	var respErr *vault.ResponseError
	if errors.As(err, &respErr) {
		return false // explicit HTTP response — not transient
	}
	return true // unknown wrapping (net.Error, EOF, etc.) — assume transient
}

// readOnce attempts a single KV v2 → v1 read. Returns (value, found, err).
func (p *OpenBAOProvider) readOnce(ctx context.Context, key string) (string, bool, error) {
	pathV2 := p.pathV2(key)
	secret, err := p.client.Logical().ReadWithContext(ctx, pathV2)
	if err == nil && secret != nil && secret.Data != nil {
		if data, ok := secret.Data["data"].(map[string]interface{}); ok {
			if v, ok := data[openbaoValueKey].(string); ok {
				return v, true, nil
			}
		} else {
			log.Printf("openbao: KV v2 secret at %q is present but missing \"data\" wrapper (malformed response)", pathV2)
		}
		return "", true, nil
	}
	if err != nil && !isNotFound(err) {
		return "", false, err
	}
	// Fallback: KV v1.
	pathV1 := p.pathV1(key)
	secret, err = p.client.Logical().ReadWithContext(ctx, pathV1)
	if err != nil {
		return "", false, err
	}
	if secret == nil || secret.Data == nil {
		return "", true, nil
	}
	if v, ok := secret.Data[openbaoValueKey].(string); ok {
		return v, true, nil
	}
	return "", true, nil
}

// Get reads a secret from OpenBao with exponential-backoff retries for transient
// errors (e.g. deadline exceeded on cold-start in low-traffic environments).
// Non-transient errors (HTTP 4xx/5xx) fail immediately without retrying.
func (p *OpenBAOProvider) Get(ctx context.Context, key string) (string, error) {
	var lastErr error
	for attempt := 0; attempt <= openbaoMaxRetries; attempt++ {
		if attempt > 0 {
			wait := openbaoRetryBaseWait * (1 << (attempt - 1)) // 2s, 4s, 8s
			log.Printf("openbao: transient error reading %q (attempt %d/%d), retrying in %s: %v",
				key, attempt, openbaoMaxRetries, wait, lastErr)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return "", formatOpenbaoError("read", key, ctx.Err())
			}
		}

		v, _, err := p.readOnce(ctx, key)
		if err == nil {
			return v, nil
		}
		if !isTransient(err) {
			return "", formatOpenbaoError("read", key, err)
		}
		lastErr = err
	}
	return "", formatOpenbaoError("read", key, lastErr)
}

func (p *OpenBAOProvider) Set(ctx context.Context, key string, value string) error {
	// Try KV v2 first: body is {"data": {"value": "..."}}.
	pathV2 := p.pathV2(key)
	_, err := p.client.Logical().WriteWithContext(ctx, pathV2, map[string]interface{}{
		"data": map[string]interface{}{openbaoValueKey: value},
	})
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		// 405 Method Not Allowed or other error: backend might be KV v1.
		var respErr *vault.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 405 {
			// Fall through to KV v1.
		} else {
			return formatOpenbaoError("write", key, err)
		}
	}
	// Fallback: KV v1 (body {"value": "..."}).
	pathV1 := p.pathV1(key)
	_, err = p.client.Logical().WriteWithContext(ctx, pathV1, map[string]interface{}{
		openbaoValueKey: value,
	})
	if err != nil {
		return formatOpenbaoError("write", key, err)
	}
	return nil
}

func (p *OpenBAOProvider) Delete(ctx context.Context, key string) error {
	pathV2 := p.pathV2(key)
	_, err := p.client.Logical().DeleteWithContext(ctx, pathV2)
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		var respErr *vault.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == 405 {
			// Fall through to KV v1.
		} else {
			return formatOpenbaoError("delete", key, err)
		}
	}
	pathV1 := p.pathV1(key)
	_, err = p.client.Logical().DeleteWithContext(ctx, pathV1)
	if err != nil {
		return formatOpenbaoError("delete", key, err)
	}
	return nil
}

func (p *OpenBAOProvider) List(ctx context.Context, prefix string) ([]string, error) {
	parseKeys := func(data map[string]interface{}) []string {
		raw, ok := data["keys"].([]interface{})
		if !ok || len(raw) == 0 {
			return nil
		}
		keys := make([]string, 0, len(raw))
		for _, k := range raw {
			if s, ok := k.(string); ok {
				keys = append(keys, strings.TrimSuffix(s, "/"))
			}
		}
		return keys
	}
	// Try KV v2 list path: secret/metadata/prefix.
	pathV2 := p.pathV2List(prefix)
	secret, err := p.client.Logical().ListWithContext(ctx, pathV2)
	if err == nil && secret != nil && secret.Data != nil {
		// A successful v2 read is definitive: return its keys (possibly empty).
		return parseKeys(secret.Data), nil
	}
	if err != nil && !isNotFound(err) {
		return nil, formatOpenbaoError("list", prefix, err)
	}
	// Fallback: KV v1 list path (secret/prefix).
	pathV1 := p.pathV1(prefix)
	secret, err = p.client.Logical().ListWithContext(ctx, pathV1)
	if err != nil {
		return nil, formatOpenbaoError("list", prefix, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}
	return parseKeys(secret.Data), nil
}

var _ SecretsProvider = (*OpenBAOProvider)(nil)
