package secrets

import (
	"context"
	"fmt"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
)

const (
	openbaoTimeout  = 15 * time.Second
	openbaoValueKey = "value"
)

// OpenBAOProvider implements SecretsProvider using the HashiCorp Vault API client.
// OpenBao is API-compatible with Vault, so the official SDK works against both.
type OpenBAOProvider struct {
	client *vault.Client
	mount  string
}

// NewOpenBAOProvider creates a secrets provider that talks to OpenBao (or Vault) at address
// using the given token. Mount is the KV v1 secrets engine path (e.g. "secret").
func NewOpenBAOProvider(address, token, mount string) (*OpenBAOProvider, error) {
	if mount == "" {
		mount = "secret"
	}
	address = strings.TrimSuffix(address, "/")

	cfg := vault.DefaultConfig()
	cfg.Address = address
	cfg.HttpClient.Timeout = openbaoTimeout

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

func (p *OpenBAOProvider) path(key string) string {
	if key == "" {
		return strings.TrimSuffix(p.mount, "/") + "/"
	}
	return p.mount + "/" + key
}

func (p *OpenBAOProvider) Get(ctx context.Context, key string) (string, error) {
	secret, err := p.client.Logical().ReadWithContext(ctx, p.path(key))
	if err != nil {
		return "", fmt.Errorf("openbao read: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return "", nil
	}
	if v, ok := secret.Data[openbaoValueKey].(string); ok {
		return v, nil
	}
	return "", nil
}

func (p *OpenBAOProvider) Set(ctx context.Context, key string, value string) error {
	_, err := p.client.Logical().WriteWithContext(ctx, p.path(key), map[string]interface{}{
		openbaoValueKey: value,
	})
	if err != nil {
		return fmt.Errorf("openbao write: %w", err)
	}
	return nil
}

func (p *OpenBAOProvider) Delete(ctx context.Context, key string) error {
	_, err := p.client.Logical().DeleteWithContext(ctx, p.path(key))
	if err != nil {
		return fmt.Errorf("openbao delete: %w", err)
	}
	return nil
}

func (p *OpenBAOProvider) List(ctx context.Context, prefix string) ([]string, error) {
	path := p.path(prefix)
	secret, err := p.client.Logical().ListWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("openbao list: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}
	raw, ok := secret.Data["keys"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, nil
	}
	keys := make([]string, 0, len(raw))
	for _, k := range raw {
		if s, ok := k.(string); ok {
			keys = append(keys, s)
		}
	}
	return keys, nil
}

var _ SecretsProvider = (*OpenBAOProvider)(nil)
