package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
)

type pluginConfig struct {
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	Mount     string `json:"mount,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

type getInput struct {
	Key    string       `json:"key"`
	Config pluginConfig `json:"config"`
}

type setInput struct {
	Key    string       `json:"key"`
	Value  string       `json:"value"`
	Config pluginConfig `json:"config"`
}

type deleteInput struct {
	Key    string       `json:"key"`
	Config pluginConfig `json:"config"`
}

type listInput struct {
	Prefix string       `json:"prefix"`
	Config pluginConfig `json:"config"`
}

type getOutput struct {
	Value string `json:"value,omitempty"`
	Found *bool  `json:"found,omitempty"`
	Error string `json:"error,omitempty"`
}

type okOutput struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type listOutput struct {
	Keys  []string `json:"keys"`
	Error string   `json:"error,omitempty"`
}

type provider struct {
	baseURL string
	token   string
	mount   string
	client  *http.Client
}

//go:wasmexport get_name
func getName() int32 {
	pdk.OutputString("openlobster-secrets-openbao")
	return 0
}

//go:wasmexport get_version
func getVersion() int32 {
	pdk.OutputString("0.1.0")
	return 0
}

//go:wasmexport get_description
func getDescription() int32 {
	pdk.OutputString("OpenBao/Vault secrets provider for OpenLobster")
	return 0
}

//go:wasmexport get_type
func getType() int32 {
	pdk.OutputString("secrets")
	return 0
}

//go:wasmexport get_schema
func getSchema() int32 {
	pdk.OutputString(`{"type":"object","properties":{"url":{"type":"string","title":"OpenBao URL","default":"http://localhost:8200","description":"Base URL of the OpenBao/Vault server"},"token":{"type":"string","title":"OpenBao Token","description":"Access token with read/write permissions on the selected mount"},"mount":{"type":"string","title":"Secrets Mount","default":"secret","description":"KV mount name where secrets are stored"},"timeout_ms":{"type":"integer","title":"HTTP Timeout (ms)","default":5000,"description":"HTTP request timeout in milliseconds"}},"required":["url","token"],"additionalProperties":false}`)
	return 0
}

//go:wasmexport get
func getSecret() int32 {
	var in getInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}
	p, err := newProvider(in.Config)
	if err != nil {
		return writeJSON(getOutput{Error: err.Error()})
	}

	value, found, err := p.get(in.Key)
	if err != nil {
		return writeJSON(getOutput{Error: err.Error()})
	}
	if !found {
		f := false
		return writeJSON(getOutput{Found: &f})
	}
	f := true
	return writeJSON(getOutput{Value: value, Found: &f})
}

//go:wasmexport set
func setSecret() int32 {
	var in setInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}
	p, err := newProvider(in.Config)
	if err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	if err := p.set(in.Key, in.Value); err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	return writeJSON(okOutput{OK: true})
}

//go:wasmexport delete
func deleteSecret() int32 {
	var in deleteInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}
	p, err := newProvider(in.Config)
	if err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	if err := p.delete(in.Key); err != nil {
		return writeJSON(okOutput{OK: false, Error: err.Error()})
	}
	return writeJSON(okOutput{OK: true})
}

//go:wasmexport list
func listSecrets() int32 {
	var in listInput
	if err := pdk.InputJSON(&in); err != nil {
		pdk.SetError(err)
		return 1
	}
	p, err := newProvider(in.Config)
	if err != nil {
		return writeJSON(listOutput{Error: err.Error()})
	}
	keys, err := p.list(in.Prefix)
	if err != nil {
		return writeJSON(listOutput{Error: err.Error()})
	}
	return writeJSON(listOutput{Keys: keys})
}

func newProvider(cfg pluginConfig) (*provider, error) {
	url := strings.TrimSpace(cfg.URL)
	token := strings.TrimSpace(cfg.Token)
	mount := strings.Trim(strings.TrimSpace(cfg.Mount), "/")
	if mount == "" {
		mount = "secret"
	}
	if url == "" {
		return nil, fmt.Errorf("config.url is required")
	}
	if token == "" {
		return nil, fmt.Errorf("config.token is required")
	}
	timeout := 5000
	if cfg.TimeoutMs > 0 {
		timeout = cfg.TimeoutMs
	}
	return &provider{
		baseURL: strings.TrimSuffix(url, "/"),
		token:   token,
		mount:   mount,
		client:  &http.Client{Timeout: time.Duration(timeout) * time.Millisecond},
	}, nil
}

func (p *provider) get(key string) (string, bool, error) {
	k := normalizeKey(key)
	if k == "" {
		return "", false, fmt.Errorf("key is required")
	}

	status, body, err := p.request(http.MethodGet, "data/"+k, map[string]any{})
	if err == nil {
		if v, ok := parseKV2Value(body); ok {
			return v, true, nil
		}
		return "", true, nil
	}
	if status != http.StatusNotFound {
		return "", false, parseHTTPError("get", status, body, err)
	}

	status, body, err = p.request(http.MethodGet, k, map[string]any{})
	if err == nil {
		if v, ok := parseKV1Value(body); ok {
			return v, true, nil
		}
		return "", true, nil
	}
	if status == http.StatusNotFound {
		return "", false, nil
	}
	return "", false, parseHTTPError("get", status, body, err)
}

func (p *provider) set(key, value string) error {
	k := normalizeKey(key)
	if k == "" {
		return fmt.Errorf("key is required")
	}

	status, body, err := p.request(http.MethodPost, "data/"+k, map[string]any{"data": map[string]any{"value": value}})
	if err == nil {
		return nil
	}
	if status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		return parseHTTPError("set", status, body, err)
	}

	status, body, err = p.request(http.MethodPost, k, map[string]any{"value": value})
	if err == nil {
		return nil
	}
	return parseHTTPError("set", status, body, err)
}

func (p *provider) delete(key string) error {
	k := normalizeKey(key)
	if k == "" {
		return fmt.Errorf("key is required")
	}

	status, body, err := p.request(http.MethodDelete, "data/"+k, nil)
	if err == nil {
		return nil
	}
	if status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		return parseHTTPError("delete", status, body, err)
	}

	status, body, err = p.request(http.MethodDelete, k, nil)
	if err == nil || status == http.StatusNotFound {
		return nil
	}
	return parseHTTPError("delete", status, body, err)
}

func (p *provider) list(prefix string) ([]string, error) {
	k := strings.Trim(strings.TrimSpace(prefix), "/")

	status, body, err := p.listPath("metadata/" + k)
	if err == nil {
		return parseListKeys(body), nil
	}
	if status != http.StatusNotFound {
		return nil, parseHTTPError("list", status, body, err)
	}

	status, body, err = p.listPath(k)
	if err == nil {
		return parseListKeys(body), nil
	}
	if status == http.StatusNotFound {
		return []string{}, nil
	}
	return nil, parseHTTPError("list", status, body, err)
}

func (p *provider) listPath(suffix string) (int, []byte, error) {
	status, body, err := p.request("LIST", suffix, nil)
	if err == nil {
		return status, body, nil
	}
	if status == http.StatusMethodNotAllowed {
		return p.request(http.MethodGet, suffix+"?list=true", nil)
	}
	return status, body, err
}

func (p *provider) request(method, suffix string, payload map[string]any) (int, []byte, error) {
	url := p.endpoint(suffix)

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-Vault-Token", p.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return resp.StatusCode, nil, readErr
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, respBody, nil
	}
	return resp.StatusCode, respBody, fmt.Errorf("http status %d", resp.StatusCode)
}

func (p *provider) endpoint(suffix string) string {
	sfx := strings.TrimPrefix(suffix, "/")
	if sfx == "" {
		return p.baseURL + "/v1/" + p.mount
	}
	return p.baseURL + "/v1/" + p.mount + "/" + sfx
}

func normalizeKey(key string) string {
	return strings.Trim(strings.TrimSpace(key), "/")
}

func parseKV2Value(body []byte) (string, bool) {
	var payload struct {
		Data struct {
			Data map[string]any `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	v, ok := payload.Data.Data["value"].(string)
	return v, ok
}

func parseKV1Value(body []byte) (string, bool) {
	var payload struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}
	v, ok := payload.Data["value"].(string)
	return v, ok
}

func parseListKeys(body []byte) []string {
	var payload struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return []string{}
	}
	keys := make([]string, 0, len(payload.Data.Keys))
	for _, k := range payload.Data.Keys {
		keys = append(keys, strings.TrimSuffix(k, "/"))
	}
	return keys
}

func parseHTTPError(op string, status int, body []byte, err error) error {
	if err == nil {
		return nil
	}
	var payload struct {
		Errors []string `json:"errors"`
	}
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil && len(payload.Errors) > 0 {
		return fmt.Errorf("openbao %s failed (status %d): %s", op, status, strings.Join(payload.Errors, "; "))
	}
	if status > 0 {
		return fmt.Errorf("openbao %s failed (status %d)", op, status)
	}
	return fmt.Errorf("openbao %s failed: %w", op, err)
}

func writeJSON(v any) int32 {
	if err := pdk.OutputJSON(v); err != nil {
		pdk.SetError(err)
		return 1
	}
	return 0
}

func main() {}
