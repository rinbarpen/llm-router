# API Doc Design

## Purpose

`API Doc` is the product-facing API reference for LLM Router. It should provide practical examples and endpoint descriptions for developers using the router.

## Primary Users

- Developers integrating with LLM Router.
- Admins validating available endpoints.
- Operators sharing integration guidance.

## Layout

Use a reference-documentation layout:

- Overview and base URL/auth section.
- Endpoint category navigation.
- Endpoint cards with method, path, description, request shape, response shape, and examples.
- Copyable code snippets.
- Error format reference.
- Links back to Help for conceptual workflows.

The page should be more structured than Help and more readable than raw API markdown.

## Key Components

- Endpoint category list.
- Endpoint detail cards.
- Copyable curl examples.
- Auth explanation.
- Error response reference.
- Cross-links to Help and Token Management.

## Data Sources

Use static frontend content based on existing repository documentation and backend behavior, especially `docs/API.md`.

For v1, do not introduce a full OpenAPI renderer unless one already exists in the repository.

## Suggested API Sections

- Authentication.
- Base URL and proxy behavior.
- Chat completions.
- Model routing.
- Models and providers.
- API keys.
- Monitor/statistics endpoints.
- Error responses.

## Empty, Loading, And Error States

Because v1 is static, no network loading state is required. If future generated API docs fail to load, show a static fallback and link to repository docs.

## Primary Interactions

- Browse endpoint categories.
- Copy code snippets.
- Jump from API key docs to Token Management.
- Jump from Chat completion docs to Chat.

## Responsive Behavior

- Desktop: side category navigation plus main reference body.
- Mobile: category navigation collapses into a sticky selector or section list.

## Acceptance Criteria

- API Doc is reachable from top navigation.
- It explains authentication and base URL usage.
- It includes practical examples for key endpoints.
- It links to Help for conceptual setup.
- It does not require backend data for v1.

