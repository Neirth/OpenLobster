// Package wasm provides the Extism-based WASM plugin runtime.
package wasm

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	extism "github.com/extism/go-sdk"
	"github.com/tetratelabs/wazero"
	"golang.org/x/crypto/ssh"
)

// Runtime stores host callbacks used for plugin instantiation.
type Runtime struct {
	onMessage func([]byte)
}

// NewRuntime builds the plugin runtime facade.
func NewRuntime(ctx context.Context, onMessage func([]byte)) (*Runtime, error) {
	_ = ctx
	return &Runtime{onMessage: onMessage}, nil
}

func (rt *Runtime) hostFunctions() []extism.HostFunction {
	if rt == nil || rt.onMessage == nil {
		return nil
	}
	emit := extism.NewHostFunctionWithStack(
		"emit_message",
		func(ctx context.Context, p *extism.CurrentPlugin, stack []uint64) {
			offset := stack[0]
			payload, err := p.ReadBytes(offset)
			if err != nil {
				p.Logf(extism.LogLevelError, "host emit_message read failed: %v", err)
				return
			}
			if len(payload) == 0 {
				return
			}
			rt.onMessage(payload)
		},
		[]extism.ValueType{extism.ValueTypeI64},
		[]extism.ValueType{},
	)
	emit.SetNamespace("openlobster")
	return []extism.HostFunction{emit}
}

func (rt *Runtime) newPluginFromPath(ctx context.Context, wasmPath string, callTimeoutMs uint64, allowFS bool, dataDir string) (*extism.Plugin, error) {
	return rt.newPluginFromWasm(
		ctx,
		extism.WasmFile{Path: wasmPath},
		callTimeoutMs,
		allowFS,
		dataDir,
		moduleStem(wasmPath),
	)
}

func (rt *Runtime) newPluginFromData(ctx context.Context, wasmName string, wasmData []byte, callTimeoutMs uint64, allowFS bool, dataDir string) (*extism.Plugin, error) {
	return rt.newPluginFromWasm(
		ctx,
		extism.WasmData{Name: wasmName, Data: wasmData},
		callTimeoutMs,
		allowFS,
		dataDir,
		moduleStem(wasmName),
	)
}

func (rt *Runtime) newPluginFromWasm(ctx context.Context, wasm extism.Wasm, callTimeoutMs uint64, allowFS bool, dataDir string, pluginID string) (*extism.Plugin, error) {
	_ = callTimeoutMs
	pluginHome := ensurePluginHomeDir(dataDir, pluginID)
	guestPluginHome := "plugin-home"
	certFile := firstExistingPath([]string{
		"/etc/ssl/cert.pem",
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/private/etc/ssl/cert.pem",
	})
	certDir := firstExistingDir([]string{
		"/etc/ssl/certs",
		"/etc/pki/tls/certs",
		"/system/etc/security/cacerts",
	})
	ollamaAuth := strings.TrimSpace(os.Getenv("OLLAMA_AUTH"))
	if ollamaAuth == "" {
		ollamaAuth = "false"
	}

	manifest := extism.Manifest{
		Wasm:         []extism.Wasm{wasm},
		AllowedHosts: []string{"*"},
	}
	if allowedPaths := buildAllowedPaths(allowFS, dataDir, pluginHome, guestPluginHome); len(allowedPaths) > 0 {
		manifest.AllowedPaths = allowedPaths
	}

	cfg := extism.PluginConfig{
		EnableWasi: false,
	}
	compiled, err := extism.NewCompiledPlugin(ctx, manifest, cfg, rt.hostFunctions())
	if err != nil {
		return nil, fmt.Errorf("extism new plugin: %w", err)
	}

	if err := instantiateExtendedWASI(ctx, compiled, map[string]string{
		"HOME":          guestPluginHome,
		"SSL_CERT_FILE": certFile,
		"SSL_CERT_DIR":  certDir,
		"OLLAMA_AUTH":   ollamaAuth,
	}); err != nil {
		_ = compiled.Close(ctx)
		return nil, fmt.Errorf("extism wasi setup: %w", err)
	}

	moduleCfg := wazero.NewModuleConfig().
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep().
		WithRandSource(rand.Reader)

	moduleCfg = moduleCfg.WithEnv("HOME", guestPluginHome)

	if certFile != "" {
		moduleCfg = moduleCfg.WithEnv("SSL_CERT_FILE", certFile)
	}
	if certDir != "" {
		moduleCfg = moduleCfg.WithEnv("SSL_CERT_DIR", certDir)
	}
	moduleCfg = moduleCfg.WithEnv("OLLAMA_AUTH", ollamaAuth)

	p, err := compiled.Instance(ctx, extism.PluginInstanceConfig{ModuleConfig: moduleCfg})
	if err != nil {
		_ = compiled.Close(ctx)
		return nil, fmt.Errorf("extism instance: %w", err)
	}

	if err := appendPluginCloser(p, compiled.Close); err != nil {
		_ = p.Close(ctx)
		_ = compiled.Close(ctx)
		return nil, fmt.Errorf("extism plugin closer setup: %w", err)
	}

	return p, nil
}

func buildAllowedPaths(allowFS bool, dataDir, pluginHome, guestPluginHome string) map[string]string {
	allowed := map[string]string{}

	addIfExists := func(path string) {
		cleanPath := filepath.Clean(strings.TrimSpace(path))
		if cleanPath == "" || cleanPath == "." {
			return
		}
		if _, err := os.Stat(cleanPath); err != nil {
			return
		}
		allowed[cleanPath] = cleanPath
	}

	addIfExistsAs := func(hostPath, guestPath string) {
		cleanHostPath := filepath.Clean(strings.TrimSpace(hostPath))
		cleanGuestPath := filepath.Clean(strings.TrimSpace(guestPath))
		if cleanHostPath == "" || cleanHostPath == "." || cleanGuestPath == "" || cleanGuestPath == "." {
			return
		}
		if _, err := os.Stat(cleanHostPath); err != nil {
			return
		}
		allowed[cleanHostPath] = cleanGuestPath
	}

	// DNS/hosts resolution for networking plugins running inside WASI.
	for _, systemPath := range []string{
		"/etc",
		"/private/etc",
		"/etc/ssl",
		"/private/etc/ssl",
	} {
		addIfExists(systemPath)
	}

	// Make /tmp writable inside the guest for SDKs that use os.TempDir() fallbacks.
	addIfExistsAs("/tmp", "/tmp")
	addIfExistsAs(os.TempDir(), "/tmp")

	// Always expose the plugin home sandbox at a stable guest path.
	addIfExistsAs(pluginHome, guestPluginHome)

	if allowFS {
		addIfExists(dataDir)
	}

	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func firstExistingPath(paths []string) string {
	for _, p := range paths {
		cleanPath := filepath.Clean(strings.TrimSpace(p))
		if cleanPath == "" || cleanPath == "." {
			continue
		}
		info, err := os.Stat(cleanPath)
		if err != nil || info.IsDir() {
			continue
		}
		return cleanPath
	}
	return ""
}

func firstExistingDir(paths []string) string {
	for _, p := range paths {
		cleanPath := filepath.Clean(strings.TrimSpace(p))
		if cleanPath == "" || cleanPath == "." {
			continue
		}
		info, err := os.Stat(cleanPath)
		if err != nil || !info.IsDir() {
			continue
		}
		return cleanPath
	}
	return ""
}

func ensurePluginHomeDir(dataDir, pluginID string) string {
	baseDir := strings.TrimSpace(dataDir)
	if baseDir == "" {
		baseDir = os.TempDir()
	}

	cleanPluginID := sanitizePathSegment(pluginID)
	if cleanPluginID == "" {
		cleanPluginID = "plugin"
	}

	homeDir := filepath.Join(baseDir, "plugin-home", cleanPluginID)
	if err := os.MkdirAll(homeDir, 0o700); err == nil {
		ensureOllamaSigningKeys(homeDir)
		return homeDir
	}

	fallback := filepath.Join(os.TempDir(), "openlobster-plugin-home", cleanPluginID)
	if err := os.MkdirAll(fallback, 0o700); err == nil {
		ensureOllamaSigningKeys(fallback)
		return fallback
	}

	return os.TempDir()
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func ensureOllamaSigningKeys(targetHome string) {
	targetHome = strings.TrimSpace(targetHome)
	if targetHome == "" {
		return
	}

	targetDir := filepath.Join(targetHome, ".ollama")
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return
	}

	privatePath := filepath.Join(targetDir, "id_ed25519")
	publicPath := filepath.Join(targetDir, "id_ed25519.pub")

	hostHome := strings.TrimSpace(os.Getenv("HOME"))
	if hostHome == "" {
		if resolvedHome, err := os.UserHomeDir(); err == nil {
			hostHome = strings.TrimSpace(resolvedHome)
		}
	}
	if hostHome != "" {
		sourceDir := filepath.Join(hostHome, ".ollama")
		files := []struct {
			name string
			mode os.FileMode
		}{
			{name: "id_ed25519", mode: 0o600},
			{name: "id_ed25519.pub", mode: 0o644},
		}
		for _, file := range files {
			targetPath := filepath.Join(targetDir, file.name)
			if _, err := os.Stat(targetPath); err == nil {
				continue
			}
			sourcePath := filepath.Join(sourceDir, file.name)
			contents, err := os.ReadFile(sourcePath)
			if err != nil {
				continue
			}
			_ = os.WriteFile(targetPath, contents, file.mode)
		}
	}

	if _, err := os.Stat(privatePath); err == nil {
		if _, pubErr := os.Stat(publicPath); pubErr == nil {
			return
		}
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return
	}

	privateKey, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return
	}

	if err := os.WriteFile(privatePath, pem.EncodeToMemory(privateKey), 0o600); err != nil {
		return
	}

	pubKey, err := ssh.NewPublicKey(priv.Public())
	if err != nil {
		return
	}
	_ = os.WriteFile(publicPath, ssh.MarshalAuthorizedKey(pubKey), 0o644)
}
