---
description: Delete a user and all their associated data permanently from the Chat view.
icon: shield-halved
---

# User Moderation

The Chat view includes a moderation action that lets operators permanently delete a user and all data associated with them. This is an irreversible operation and should only be performed with full certainty.

## What gets deleted

When you delete a user, the following data is removed permanently:

* All messages in the conversation
* All conversation records with that user
* Tool permissions and explicit allow/deny entries assigned to them
* The user account and all channel bindings

{% hint style="danger" %}
This action cannot be undone. There is no recovery path once the deletion is confirmed. Back up any data you need before proceeding.
{% endhint %}

## How to delete a user

{% stepper %}
{% step %}

## Open the conversation

Select the user's conversation from the conversations list on the left.

{% endstep %}

{% step %}

## Click the Delete User button

In the message thread header, click the **Delete User** button (person with a minus icon). A confirmation modal will appear listing exactly what will be deleted.

{% endstep %}

{% step %}

## Read the consequences

Review the list of data that will be removed. Make sure you are deleting the correct user.

{% endstep %}

{% step %}

## Type the user's name to confirm

In the confirmation field, type the user's display name exactly as shown. This prevents accidental deletions.

{% endstep %}

{% step %}

## Confirm the deletion

Click **Delete permanently**. Wait for the success feedback before navigating away.

{% endstep %}
{% endstepper %}

## Safety notes

* Only operators with the appropriate role should perform user deletions.
* If you need to remove multiple users, consider exporting conversation data first.
* Deleting a user does not block them from contacting the agent again in the future. If they message through a connected channel, a new conversation and user record will be created and they will need to go through the pairing process again.
