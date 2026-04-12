# Token Management Design

## Purpose

`令牌管理` is the operational surface for managing system API keys and access rules. It should reframe the existing API key management functionality as a first-class token control page.

## Primary Users

- Administrators creating and disabling access tokens.
- Operators reviewing quota and expiry settings.
- Developers copying or validating issued keys.

## Layout

Use a two-level page structure:

- Page header with token health summary: active tokens, inactive tokens, expiring soon, and monthly quota pressure.
- Main table for API keys.
- Create/edit modal or drawer for key details.
- Optional side panel or expandable details for restrictions and parameter limits.

The page should feel precise and security-focused. Tables can remain dense, but surrounding layout should be cleaner than the current generic management tab.

## Key Components

- Active/inactive toggle.
- Search by name, key fragment, or ID.
- API key table.
- Create API key action.
- Edit key form.
- Disable confirmation.
- Copy and reveal controls.
- Parameter limits editor.
- Model/provider allowlist controls.

## Data Sources

Use the existing API key service:

- List API keys.
- Create API key.
- Update API key.
- Disable/delete API key.

No backend API changes are required for v1.

## Empty, Loading, And Error States

- Loading: table loading state plus disabled destructive actions.
- Empty: explain that no tokens exist and provide a create-token CTA.
- Error: show API error message via existing error utilities.

## Primary Interactions

- Create token.
- Edit token name, expiry, quota, allowlists, and parameter limits.
- Reveal token temporarily.
- Copy token.
- Disable token after confirmation.
- Include inactive tokens.

## Security Requirements

- Tokens must be masked by default.
- Copy and reveal actions must be explicit.
- Newly created keys should be shown with a clear save-now warning.
- Destructive actions require confirmation.

## Responsive Behavior

- Desktop: full table with action buttons.
- Mobile: table may scroll horizontally or transform to cards, but copy/reveal/disable actions must remain available.

## Acceptance Criteria

- Existing API key behavior remains available.
- Secret masking is preserved.
- Create/edit/disable flows still call existing APIs.
- The page is accessible from the left operations navigation as `令牌管理`.

