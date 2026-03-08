package secrets

import (
	"context"
	"fmt"
)

type OpenBAOProvider struct {
	client *OpenBAOClient
	mount  string
}

type OpenBAOClient struct {
	address string
	token   string
}

func NewOpenBAOProvider(address, token, mount string) (*OpenBAOProvider, error) {
	if mount == "" {
		mount = "secret"
	}

	client := &OpenBAOClient{
		address: address,
		token:   token,
	}

	return &OpenBAOProvider{
		client: client,
		mount:  mount,
	}, nil
}

func (p *OpenBAOProvider) Get(ctx context.Context, key string) (string, error) {
	return p.client.Get(p.mount, key)
}

func (p *OpenBAOProvider) Set(ctx context.Context, key string, value string) error {
	return p.client.Set(p.mount, key, value)
}

func (p *OpenBAOProvider) Delete(ctx context.Context, key string) error {
	return p.client.Delete(p.mount, key)
}

func (p *OpenBAOProvider) List(ctx context.Context, prefix string) ([]string, error) {
	return p.client.List(p.mount, prefix)
}

func (c *OpenBAOClient) Get(mount, key string) (string, error) {
	return "", fmt.Errorf("OpenBAO client not initialized - requires openbao library")
}

func (c *OpenBAOClient) Set(mount, key, value string) error {
	return fmt.Errorf("OpenBAO client not initialized")
}

func (c *OpenBAOClient) Delete(mount, key string) error {
	return fmt.Errorf("OpenBAO client not initialized")
}

func (c *OpenBAOClient) List(mount, prefix string) ([]string, error) {
	return nil, fmt.Errorf("OpenBAO client not initialized")
}

var _ SecretsProvider = (*OpenBAOProvider)(nil)
