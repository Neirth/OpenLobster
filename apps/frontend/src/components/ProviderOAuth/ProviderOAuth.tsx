// Copyright (c) OpenLobster contributors. See LICENSE for details.

import type { Component } from "solid-js";
import { createSignal, For, Show, onMount, onCleanup } from "solid-js";
import { getStoredToken, setNeedsAuth } from "../../stores/authStore";
import { GRAPHQL_ENDPOINT } from "../../graphql/client";
import {
  PROVIDER_OAUTH_PROVIDERS_QUERY,
  PROVIDER_OAUTH_STATUS_QUERY,
  PROVIDER_OAUTH_PROFILES_QUERY,
  INITIATE_PROVIDER_OAUTH_MUTATION,
  LOGOUT_PROVIDER_OAUTH_MUTATION,
  SET_ACTIVE_OAUTH_PROFILE_MUTATION,
  DELETE_OAUTH_PROFILE_MUTATION,
} from "@openlobster/ui/graphql/mutations";
import "./ProviderOAuth.css";

interface OAuthProvider {
  id: string;
  name: string;
}

interface OAuthProfile {
  name: string;
  authenticated: boolean;
  accountID: string;
}

interface OAuthStatus {
  provider: string;
  status: string;
  errorMessage: string;
}

function graphqlHeaders(): Record<string, string> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getStoredToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;
  return headers;
}

async function gqlFetch<T>(query: string, variables?: Record<string, unknown>): Promise<T> {
  const res = await fetch(GRAPHQL_ENDPOINT, {
    method: "POST",
    headers: graphqlHeaders(),
    body: JSON.stringify({ query, variables }),
  });
  if (res.status === 401) {
    setNeedsAuth(true);
    throw new Error("Unauthorized");
  }
  const data = await res.json();
  if (data.errors?.length) {
    throw new Error(data.errors[0]?.message ?? "GraphQL error");
  }
  return data.data as T;
}

const ProviderOAuth: Component = () => {
  const [providers, setProviders] = createSignal<OAuthProvider[]>([]);
  const [profiles, setProfiles] = createSignal<Record<string, OAuthProfile[]>>({});
  const [statuses, setStatuses] = createSignal<Record<string, OAuthStatus>>({});
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [polling, setPolling] = createSignal<string | null>(null);
  const [instructions, setInstructions] = createSignal<string | null>(null);
  const [newProfileName, setNewProfileName] = createSignal<Record<string, string>>({});
  const [actionLoading, setActionLoading] = createSignal<string | null>(null);

  let pollInterval: ReturnType<typeof setInterval> | null = null;

  onCleanup(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  const fetchProviders = async () => {
    try {
      setLoading(true);
      setError(null);
      const data = await gqlFetch<{ providerOAuthProviders: OAuthProvider[] }>(
        PROVIDER_OAUTH_PROVIDERS_QUERY,
      );
      setProviders(data.providerOAuthProviders ?? []);

      // Fetch profiles and status for each provider
      const providerList = data.providerOAuthProviders ?? [];
      for (const provider of providerList) {
        await fetchProviderData(provider.id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load providers");
    } finally {
      setLoading(false);
    }
  };

  const fetchProviderData = async (providerId: string) => {
    try {
      const [statusData, profileData] = await Promise.all([
        gqlFetch<{ providerOAuthStatus: OAuthStatus }>(
          PROVIDER_OAUTH_STATUS_QUERY,
          { provider: providerId },
        ),
        gqlFetch<{ providerOAuthProfiles: OAuthProfile[] }>(
          PROVIDER_OAUTH_PROFILES_QUERY,
          { provider: providerId },
        ),
      ]);

      setStatuses((prev) => ({
        ...prev,
        [providerId]: statusData.providerOAuthStatus,
      }));
      setProfiles((prev) => ({
        ...prev,
        [providerId]: profileData.providerOAuthProfiles ?? [],
      }));
    } catch {
      // Silently handle per-provider fetch failures
    }
  };

  const startOAuthFlow = async (providerId: string, profileName?: string) => {
    try {
      setActionLoading(providerId);
      setError(null);
      setInstructions(null);

      const data = await gqlFetch<{
        initiateProviderOAuth: { authorizationURL: string; instructions: string };
      }>(INITIATE_PROVIDER_OAUTH_MUTATION, {
        provider: providerId,
        profileName: profileName || undefined,
      });

      const result = data.initiateProviderOAuth;
      if (result.instructions) {
        setInstructions(result.instructions);
      }
      if (result.authorizationURL) {
        window.open(result.authorizationURL, "oauth_popup", "width=600,height=700");
        setPolling(providerId);
        startPolling(providerId);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "OAuth initiation failed");
    } finally {
      setActionLoading(null);
    }
  };

  const startPolling = (providerId: string) => {
    if (pollInterval) clearInterval(pollInterval);
    pollInterval = setInterval(async () => {
      try {
        const data = await gqlFetch<{ providerOAuthStatus: OAuthStatus }>(
          PROVIDER_OAUTH_STATUS_QUERY,
          { provider: providerId },
        );
        const status = data.providerOAuthStatus;
        setStatuses((prev) => ({ ...prev, [providerId]: status }));

        if (status.status === "authenticated") {
          stopPolling();
          setPolling(null);
          setInstructions(null);
          await fetchProviderData(providerId);
        } else if (status.errorMessage) {
          stopPolling();
          setPolling(null);
          setInstructions(null);
          setError(status.errorMessage);
        }
      } catch {
        // Continue polling on transient errors
      }
    }, 2000);
  };

  const stopPolling = () => {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  };

  const handleLogout = async (providerId: string) => {
    try {
      setActionLoading(providerId);
      await gqlFetch<{ logoutProviderOAuth: boolean }>(
        LOGOUT_PROVIDER_OAUTH_MUTATION,
        { provider: providerId },
      );
      await fetchProviderData(providerId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Logout failed");
    } finally {
      setActionLoading(null);
    }
  };

  const handleSetActiveProfile = async (providerId: string, profileName: string) => {
    try {
      setActionLoading(`${providerId}:${profileName}`);
      await gqlFetch<{ setActiveOAuthProfile: boolean }>(
        SET_ACTIVE_OAUTH_PROFILE_MUTATION,
        { provider: providerId, profileName },
      );
      await fetchProviderData(providerId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to switch profile");
    } finally {
      setActionLoading(null);
    }
  };

  const handleDeleteProfile = async (providerId: string, profileName: string) => {
    try {
      setActionLoading(`${providerId}:delete:${profileName}`);
      await gqlFetch<{ deleteOAuthProfile: boolean }>(
        DELETE_OAUTH_PROFILE_MUTATION,
        { provider: providerId, profileName },
      );
      await fetchProviderData(providerId);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete profile");
    } finally {
      setActionLoading(null);
    }
  };

  const handleAddAccount = (providerId: string) => {
    const name = (newProfileName()[providerId] ?? "").trim();
    setNewProfileName((prev) => ({ ...prev, [providerId]: "" }));
    startOAuthFlow(providerId, name || undefined);
  };

  const getStatusBadgeClass = (status?: string): string => {
    if (status === "authenticated") return "provider-oauth-badge provider-oauth-badge--authenticated";
    if (status === "expired") return "provider-oauth-badge provider-oauth-badge--expired";
    return "provider-oauth-badge provider-oauth-badge--disconnected";
  };

  const getStatusLabel = (status?: string): string => {
    if (status === "authenticated") return "Authenticated";
    if (status === "expired") return "Expired";
    return "Not connected";
  };

  const getStatusIcon = (status?: string): string => {
    if (status === "authenticated") return "check_circle";
    if (status === "expired") return "warning";
    return "cancel";
  };

  onMount(() => {
    fetchProviders();
  });

  return (
    <div class="provider-oauth">
      <Show when={loading()}>
        <div class="provider-oauth__loading">
          <span class="material-symbols-outlined">rotate_right</span>
          <p>Loading OAuth providers...</p>
        </div>
      </Show>

      <Show when={!loading() && providers().length === 0}>
        <p class="provider-oauth__empty">No OAuth providers available.</p>
      </Show>

      <Show when={error()}>
        <p class="provider-oauth-error">{error()}</p>
      </Show>

      <Show when={!loading()}>
        <For each={providers()}>
          {(provider) => {
            const status = () => statuses()[provider.id];
            const providerProfiles = () => profiles()[provider.id] ?? [];
            const isPolling = () => polling() === provider.id;
            const isActionLoading = () => actionLoading() === provider.id;

            return (
              <div class="provider-oauth-card">
                <div class="provider-oauth-card__header">
                  <span class="provider-oauth-card__name">
                    <span class="material-symbols-outlined">key</span>
                    {provider.name}
                  </span>
                  <div class="provider-oauth-card__actions">
                    <span class={getStatusBadgeClass(status()?.status)}>
                      <span class="material-symbols-outlined">
                        {getStatusIcon(status()?.status)}
                      </span>
                      {getStatusLabel(status()?.status)}
                    </span>
                    <Show when={status()?.status === "authenticated"}>
                      <button
                        class="provider-oauth-btn provider-oauth-btn--danger"
                        onClick={() => handleLogout(provider.id)}
                        disabled={isActionLoading()}
                      >
                        <span class="material-symbols-outlined">logout</span>
                        Logout
                      </button>
                    </Show>
                    <Show when={status()?.status !== "authenticated" && !isPolling()}>
                      <button
                        class="provider-oauth-btn provider-oauth-btn--primary"
                        onClick={() => startOAuthFlow(provider.id)}
                        disabled={isActionLoading()}
                      >
                        <span class="material-symbols-outlined">login</span>
                        Sign In
                      </button>
                    </Show>
                  </div>
                </div>

                {/* Polling / waiting state */}
                <Show when={isPolling()}>
                  <div class="provider-oauth-waiting">
                    <span class="material-symbols-outlined">rotate_right</span>
                    Waiting for authentication to complete...
                  </div>
                </Show>

                {/* Instructions from the OAuth flow */}
                <Show when={instructions() && isPolling()}>
                  <div class="provider-oauth-instructions">{instructions()}</div>
                </Show>

                {/* Profile list */}
                <Show when={providerProfiles().length > 0}>
                  <div class="provider-oauth-profiles">
                    <For each={providerProfiles()}>
                      {(profile) => {
                        const isActive = () => profile.authenticated;
                        const isProfileActionLoading = () =>
                          actionLoading() === `${provider.id}:${profile.name}` ||
                          actionLoading() === `${provider.id}:delete:${profile.name}`;

                        return (
                          <div
                            class="provider-oauth-profile"
                            classList={{ "provider-oauth-profile--active": isActive() }}
                          >
                            <div class="provider-oauth-profile__info">
                              <span class="provider-oauth-profile__name">{profile.name}</span>
                              <Show when={profile.accountID}>
                                <span class="provider-oauth-profile__account">
                                  {profile.accountID}
                                </span>
                              </Show>
                            </div>
                            <span class={getStatusBadgeClass(profile.authenticated ? "authenticated" : "expired")}>
                              {profile.authenticated ? "Active" : "Inactive"}
                            </span>
                            <div class="provider-oauth-profile__actions">
                              <Show when={!isActive()}>
                                <button
                                  class="provider-oauth-btn provider-oauth-btn--secondary"
                                  onClick={() => handleSetActiveProfile(provider.id, profile.name)}
                                  disabled={isProfileActionLoading()}
                                  title="Set as active profile"
                                >
                                  <span class="material-symbols-outlined">swap_horiz</span>
                                  Activate
                                </button>
                              </Show>
                              <button
                                class="provider-oauth-btn provider-oauth-btn--icon"
                                onClick={() => handleDeleteProfile(provider.id, profile.name)}
                                disabled={isProfileActionLoading()}
                                title="Delete profile"
                              >
                                <span class="material-symbols-outlined">delete</span>
                              </button>
                            </div>
                          </div>
                        );
                      }}
                    </For>
                  </div>
                </Show>

                {/* Add account row */}
                <Show when={!isPolling()}>
                  <div class="provider-oauth-add">
                    <input
                      type="text"
                      placeholder="Profile name (optional)"
                      value={newProfileName()[provider.id] ?? ""}
                      onInput={(e) =>
                        setNewProfileName((prev) => ({
                          ...prev,
                          [provider.id]: e.currentTarget.value,
                        }))
                      }
                      onKeyDown={(e) => {
                        if (e.key === "Enter") handleAddAccount(provider.id);
                      }}
                    />
                    <button
                      class="provider-oauth-btn provider-oauth-btn--secondary"
                      onClick={() => handleAddAccount(provider.id)}
                      disabled={isActionLoading()}
                    >
                      <span class="material-symbols-outlined">person_add</span>
                      Add Account
                    </button>
                  </div>
                </Show>
              </div>
            );
          }}
        </For>
      </Show>
    </div>
  );
};

export default ProviderOAuth;
