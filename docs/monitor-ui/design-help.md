# Help Design

## Purpose

`Help` is the user-facing help center for understanding common LLM Router workflows. It should explain how to use the monitor UI, configure providers, manage tokens, use Chat, and troubleshoot common issues.

## Primary Users

- New users setting up LLM Router.
- Developers using Chat or API docs.
- Operators troubleshooting runtime behavior.

## Layout

Use a readable documentation layout:

- Intro hero with quick-start CTA.
- Common workflow cards.
- Troubleshooting section.
- FAQ section.
- Cross-links to API Doc and relevant monitor pages.

Help should be content-first and editorial. It should not look like a table-heavy operations page.

## Key Components

- Quick start checklist.
- Workflow cards.
- FAQ accordion.
- Troubleshooting cards.
- Search or filter can be added later, but is not required for v1.
- Links to `API Doc`, `模型广场`, `令牌管理`, and `Chat`.

## Data Sources

Use static frontend content for v1. Later versions may load Markdown or generated docs.

## Empty, Loading, And Error States

Because v1 is static, no network loading state is required. If content is later loaded dynamically, fallback to a static minimal help index.

## Primary Interactions

- Open quick-start steps.
- Expand FAQ/troubleshooting items.
- Navigate to API Doc.
- Navigate to relevant app pages.

## Responsive Behavior

- Desktop: article body with side contents or card grid.
- Mobile: single-column content with anchored sections.

## Acceptance Criteria

- Help is reachable from top navigation.
- It includes setup, model usage, token management, Chat usage, and troubleshooting guidance.
- It links to API Doc for request/response details.
- It does not require backend data.

