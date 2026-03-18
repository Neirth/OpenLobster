package provideroauth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/neirth/openlobster/internal/infrastructure/secrets"
)

const (
	// credentialsKeyPrefix is the prefix for storing provider OAuth credentials.
	credentialsKeyPrefix = "provider-oauth/"

	// credentialsKeySuffix is the suffix appended after the profile name.
	credentialsKeySuffix = "/credentials"

	// activeProfileKeySuffix is the suffix for storing the active profile name.
	activeProfileKeySuffix = "/active-profile"

	// defaultProfileName is the profile name used when none is specified.
	defaultProfileName = "default"

	// refreshCheckInterval is how often the background goroutine checks for
	// tokens that need refreshing.
	refreshCheckInterval = 60 * time.Second

	// refreshThreshold is how far before expiry we proactively refresh tokens.
	// Tokens are refreshed when they expire within the next 5 minutes.
	refreshThreshold = 5 * time.Minute
)

// Manager handles the OAuth credential lifecycle for AI providers.
// It stores and retrieves credentials from a SecretsProvider and can
// auto-refresh tokens before they expire via a background goroutine.
type Manager struct {
	secrets secrets.SecretsProvider

	mu     sync.RWMutex
	stopCh chan struct{}
	done   chan struct{}
}

// NewManager creates a new Manager backed by the given SecretsProvider.
func NewManager(s secrets.SecretsProvider) *Manager {
	return &Manager{
		secrets: s,
	}
}

// profileCredentialsKey returns the secrets key for a given provider ID and profile name.
// Key format: "provider-oauth/{providerID}/{profileName}/credentials"
func profileCredentialsKey(providerID, profileName string) string {
	return credentialsKeyPrefix + providerID + "/" + profileName + credentialsKeySuffix
}

// credentialsKey returns the secrets key for a given provider ID using the default profile.
// Kept for backward compatibility in tests.
func credentialsKey(providerID string) string {
	return profileCredentialsKey(providerID, defaultProfileName)
}

// activeProfileKey returns the secrets key for storing the active profile of a provider.
func activeProfileKey(providerID string) string {
	return credentialsKeyPrefix + providerID + activeProfileKeySuffix
}

// Login runs the OAuth login flow for the given provider using the "default" profile,
// stores the resulting credentials, and returns them.
func (m *Manager) Login(ctx context.Context, providerID string, callbacks LoginCallbacks) (*Credentials, error) {
	return m.LoginWithProfile(ctx, providerID, defaultProfileName, callbacks)
}

// LoginWithProfile runs the OAuth login flow for the given provider and profile,
// stores the resulting credentials, and returns them.
func (m *Manager) LoginWithProfile(ctx context.Context, providerID, profileName string, callbacks LoginCallbacks) (*Credentials, error) {
	provider := Get(providerID)
	if provider == nil {
		return nil, ErrProviderNotFound
	}

	creds, err := provider.Login(ctx, callbacks)
	if err != nil {
		return nil, fmt.Errorf("manager login %s/%s: %w", providerID, profileName, err)
	}

	if err := m.storeProfileCredentials(ctx, providerID, profileName, creds); err != nil {
		return nil, fmt.Errorf("manager login %s/%s: store: %w", providerID, profileName, err)
	}

	return creds, nil
}

// GetActiveProfile returns the currently active profile name for a provider.
// Returns "default" if no active profile is set.
func (m *Manager) GetActiveProfile(ctx context.Context, providerID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	raw, err := m.secrets.Get(ctx, activeProfileKey(providerID))
	if err != nil || raw == "" {
		return defaultProfileName
	}
	return raw
}

// SetActiveProfile sets the active profile for a provider.
func (m *Manager) SetActiveProfile(ctx context.Context, providerID, profileName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.secrets.Set(ctx, activeProfileKey(providerID), profileName)
}

// ListProfiles returns all profiles for a given provider, including authentication status.
func (m *Manager) ListProfiles(ctx context.Context, providerID string) ([]ProfileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := credentialsKeyPrefix + providerID + "/"
	keys, err := m.secrets.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("manager list profiles %s: %w", providerID, err)
	}

	var profiles []ProfileInfo
	for _, key := range keys {
		if !strings.HasSuffix(key, credentialsKeySuffix) {
			continue
		}
		// Extract profile name from key: provider-oauth/{providerID}/{profileName}/credentials
		rest := key[len(prefix):]                                    // "{profileName}/credentials"
		profileName := rest[:len(rest)-len(credentialsKeySuffix)]    // "{profileName}"
		if profileName == "" {
			continue
		}

		info := ProfileInfo{
			Name:          profileName,
			Authenticated: true,
		}

		// Try to read credentials for extra info
		raw, err := m.secrets.Get(ctx, key)
		if err == nil && raw != "" {
			var creds Credentials
			if json.Unmarshal([]byte(raw), &creds) == nil {
				info.AccountID = creds.AccountID
				if email, ok := creds.Extra["email"]; ok {
					info.Email = email
				}
			}
		} else {
			info.Authenticated = false
		}

		profiles = append(profiles, info)
	}

	return profiles, nil
}

// DeleteProfile removes the credentials for a specific profile.
func (m *Manager) DeleteProfile(ctx context.Context, providerID, profileName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.secrets.Delete(ctx, profileCredentialsKey(providerID, profileName)); err != nil {
		return fmt.Errorf("manager delete profile %s/%s: %w", providerID, profileName, err)
	}

	// If the deleted profile was the active one, reset to default.
	active, err := m.secrets.Get(ctx, activeProfileKey(providerID))
	if err == nil && active == profileName {
		_ = m.secrets.Set(ctx, activeProfileKey(providerID), defaultProfileName)
	}

	return nil
}

// GetAPIKey returns the current API key for the given provider using the active profile.
// If the token is expired or about to expire, it refreshes it first.
func (m *Manager) GetAPIKey(ctx context.Context, providerID string) (string, error) {
	provider := Get(providerID)
	if provider == nil {
		return "", ErrProviderNotFound
	}

	creds, err := m.GetCredentials(ctx, providerID)
	if err != nil {
		return "", fmt.Errorf("manager get api key %s: %w", providerID, err)
	}
	if creds == nil {
		return "", fmt.Errorf("manager get api key %s: not authenticated", providerID)
	}

	profileName := m.GetActiveProfile(ctx, providerID)

	if creds.IsExpired() || m.shouldRefresh(creds) {
		refreshed, refreshErr := provider.RefreshToken(ctx, creds)
		if refreshErr != nil {
			// If refresh fails but token is not yet expired, return it anyway.
			if !creds.IsExpired() {
				return provider.GetAPIKey(creds), nil
			}
			return "", fmt.Errorf("manager get api key %s: refresh: %w", providerID, refreshErr)
		}
		if err := m.storeProfileCredentials(ctx, providerID, profileName, refreshed); err != nil {
			return "", fmt.Errorf("manager get api key %s: store refreshed: %w", providerID, err)
		}
		creds = refreshed
	}

	return provider.GetAPIKey(creds), nil
}

// GetCredentials retrieves stored credentials for the given provider's active profile.
// Returns nil, nil if no credentials are stored.
func (m *Manager) GetCredentials(ctx context.Context, providerID string) (*Credentials, error) {
	profileName := m.GetActiveProfile(ctx, providerID)
	return m.getProfileCredentials(ctx, providerID, profileName)
}

// getProfileCredentials retrieves stored credentials for a specific profile.
func (m *Manager) getProfileCredentials(ctx context.Context, providerID, profileName string) (*Credentials, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	raw, err := m.secrets.Get(ctx, profileCredentialsKey(providerID, profileName))
	if err != nil {
		return nil, fmt.Errorf("manager get credentials %s/%s: %w", providerID, profileName, err)
	}
	if raw == "" {
		return nil, nil
	}

	var creds Credentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		return nil, fmt.Errorf("manager get credentials %s/%s: unmarshal: %w", providerID, profileName, err)
	}

	return &creds, nil
}

// IsAuthenticated reports whether valid credentials exist for the given provider's active profile.
func (m *Manager) IsAuthenticated(ctx context.Context, providerID string) bool {
	creds, err := m.GetCredentials(ctx, providerID)
	return err == nil && creds != nil
}

// Logout removes stored credentials for the given provider's active profile.
func (m *Manager) Logout(ctx context.Context, providerID string) error {
	profileName := m.GetActiveProfile(ctx, providerID)
	return m.DeleteProfile(ctx, providerID, profileName)
}

// StartAutoRefresh starts a background goroutine that periodically checks
// stored credentials and refreshes tokens that are about to expire.
func (m *Manager) StartAutoRefresh(ctx context.Context) {
	m.mu.Lock()
	if m.stopCh != nil {
		m.mu.Unlock()
		return // already running
	}
	m.stopCh = make(chan struct{})
	m.done = make(chan struct{})
	m.mu.Unlock()

	go m.refreshLoop(ctx)
}

// Stop stops the background auto-refresh goroutine and waits for it to finish.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopCh == nil {
		m.mu.Unlock()
		return
	}
	close(m.stopCh)
	done := m.done
	m.mu.Unlock()

	<-done

	m.mu.Lock()
	m.stopCh = nil
	m.done = nil
	m.mu.Unlock()
}

// storeProfileCredentials serializes and stores credentials for a specific profile.
func (m *Manager) storeProfileCredentials(ctx context.Context, providerID, profileName string, creds *Credentials) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	return m.secrets.Set(ctx, profileCredentialsKey(providerID, profileName), string(data))
}

// shouldRefresh reports whether a token should be proactively refreshed.
func (m *Manager) shouldRefresh(creds *Credentials) bool {
	return timeNowMs()+refreshThreshold.Milliseconds() >= creds.Expires
}

// refreshLoop is the background goroutine that periodically refreshes tokens.
func (m *Manager) refreshLoop(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(refreshCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshAll(ctx)
		}
	}
}

// refreshAll iterates over ALL stored credentials (all providers, all profiles)
// and refreshes any that are about to expire.
func (m *Manager) refreshAll(ctx context.Context) {
	keys, err := m.secrets.List(ctx, credentialsKeyPrefix)
	if err != nil {
		slog.Debug("provider oauth: auto-refresh list failed", "error", err)
		return
	}

	for _, key := range keys {
		if !strings.HasSuffix(key, credentialsKeySuffix) {
			continue
		}

		// Extract provider ID and profile name from key:
		// provider-oauth/{providerID}/{profileName}/credentials
		providerID, profileName := extractProviderAndProfile(key)
		if providerID == "" || profileName == "" {
			continue
		}

		provider := Get(providerID)
		if provider == nil {
			continue
		}

		creds, err := m.getProfileCredentials(ctx, providerID, profileName)
		if err != nil || creds == nil {
			continue
		}

		if !m.shouldRefresh(creds) {
			continue
		}

		refreshed, err := provider.RefreshToken(ctx, creds)
		if err != nil {
			slog.Debug("provider oauth: auto-refresh failed",
				"provider", providerID,
				"profile", profileName,
				"error", err,
			)
			continue
		}

		if err := m.storeProfileCredentials(ctx, providerID, profileName, refreshed); err != nil {
			slog.Debug("provider oauth: auto-refresh store failed",
				"provider", providerID,
				"profile", profileName,
				"error", err,
			)
		}
	}
}

// extractProviderAndProfile extracts the provider ID and profile name from a credentials key.
// Key format: "provider-oauth/{providerID}/{profileName}/credentials"
func extractProviderAndProfile(key string) (providerID, profileName string) {
	if !strings.HasPrefix(key, credentialsKeyPrefix) || !strings.HasSuffix(key, credentialsKeySuffix) {
		return "", ""
	}
	// Remove prefix and suffix
	middle := key[len(credentialsKeyPrefix) : len(key)-len(credentialsKeySuffix)]
	// middle = "{providerID}/{profileName}"
	parts := strings.SplitN(middle, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

// extractProviderID extracts the provider ID from a credentials key.
// Kept for backward compatibility. Now handles the new key format with profiles.
// Key format: "provider-oauth/{providerID}/{profileName}/credentials"
func extractProviderID(key string) string {
	id, _ := extractProviderAndProfile(key)
	return id
}
