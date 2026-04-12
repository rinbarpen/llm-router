# Profile Design

## Purpose

`个人中心` is a lightweight account and preferences page. It should not introduce full team management or IAM scope in v1.

## Primary Users

- Current monitor user.
- Admin or developer configuring local preferences.

## Layout

Use a calm account-center layout:

- Profile identity card.
- Local preferences section.
- API/token summary card.
- Recent sessions or Chat shortcuts.
- Documentation and support quick links.

This page should feel less operational and more personal than dashboard/logs, while keeping the same editorial visual system.

## Key Components

- User/account summary.
- Theme preference.
- Local UI preferences.
- API key summary derived from existing token data when available.
- Recent Chat session summary from local storage when available.
- Quick links to Help and API Doc.

## Data Sources

Use mixed/local data:

- Local storage for theme and Chat session summaries.
- Existing API key list if a lightweight token summary is needed.
- Static identity placeholder if no authenticated user profile exists.

No backend user profile system is required for v1.

## Empty, Loading, And Error States

- Empty identity: show `Local Console User` or equivalent neutral label.
- Empty recent sessions: provide CTA to open Chat.
- Error loading token summary: show the rest of the page and make the summary non-blocking.

## Primary Interactions

- Toggle theme or UI preference.
- Open Chat.
- Open token management.
- Open Help or API Doc.

## Responsive Behavior

- Desktop: profile card and summary cards in two-column layout.
- Mobile: all sections stack vertically.

## Acceptance Criteria

- Page does not require new backend auth/user APIs.
- Theme/local preferences are visible.
- Quick links work.
- The page does not expose team/member/role management.

