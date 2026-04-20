---
description: Some MCP servers require OAuth authorization before the agent can access their tools. This page explains how to complete that flow.
icon: key
---

# OAuth Flow

Some MCP servers require you to authorize the agent to act on your behalf using OAuth 2.1 before their tools become available. The authorization flow is handled entirely within the UI via a popup window.

## When you need to do this

A server that requires OAuth will show one of the following states:

* **Unauthorized** — The server is reachable but authorization has not been completed.
* A prompt or button in the **Manage** modal saying **Authorize with OAuth**.

## Completing authorization

{% stepper %}
{% step %}

## Open the Manage modal

Click **Manage** on the server that shows an authorization requirement.

{% endstep %}

{% step %}

## Click Authorize

In the modal, click the **Authorize with OAuth** button. A popup window will open and navigate to the service's authorization page.

{% endstep %}

{% step %}

## Complete the authorization in the popup

Log in to the service if prompted, then grant the requested permissions. The popup will close automatically once the authorization is complete.

{% endstep %}

{% step %}

## Confirm success

The modal will update: the server status should change to **Online** and its tools will become visible. If the authorization failed, an error message will appear in the modal.

{% endstep %}
{% endstepper %}

## Troubleshooting

{% hint style="warning" %}
If the browser blocks the authorization popup, allow popups for this site in your browser settings and try again.
{% endhint %}

* **Popup closes without success** — Return to the Manage modal and click **Authorize** again. Check the [Recent Logs](../dashboard/logs.md) panel for server-side error messages.
* **Server returns to Unauthorized after a while** — The OAuth token may have expired. Repeat the authorization flow to refresh it.
* **Redirect error page** — If you see an OAuth callback error page, the server's redirect URI may be misconfigured. Verify that the **Server Base URL** in **Settings > GraphQL** is set to the correct public URL of your OpenLobster instance, as it is used for OAuth callbacks.
