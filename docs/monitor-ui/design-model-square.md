# Model Square Design

## Purpose

`模型广场` is the product-facing model browser. It lets users discover configured and available models, inspect capabilities, and jump into Chat or configuration.

## Primary Users

- Developers choosing a model for Chat or API use.
- Admins checking provider/model availability.
- Operators validating model status.

## Layout

Use a gallery layout:

- Editorial page header explaining available model capabilities.
- Search and filter rail for provider, modality, capability, status, and tags.
- Model cards grouped or sortable by provider/capability.
- Detail drawer or expanded card for model metadata.
- Quick actions for Chat and configuration.

The page should be more product-like than `模型管理`, but it must still reflect real configuration state.

## Key Components

- Search input.
- Provider filter.
- Capability/tag filters.
- Status chips.
- Model cards.
- Pricing/capability metadata.
- Actions: `在 Chat 中使用`, `查看配置`, `同步目录` where supported.

## Data Sources

Use existing model/provider APIs where available:

- Active model list.
- Provider model data.
- Provider catalog data where already supported.
- Pricing/suggestion data where already supported.

Do not require a new backend catalog API for v1.

## Model States

Represent at least these states:

- `available`: visible and usable.
- `configured`: configured in router.
- `inactive`: known but disabled.
- `needs setup`: catalog/known model that requires provider or key setup.

## Empty, Loading, And Error States

- Loading: skeleton model cards.
- Empty: explain that no models match filters and provide reset filters.
- No configured models: link to model/provider configuration.
- Error: show retry and partial data where possible.

## Primary Interactions

- Search/filter models.
- Open model in Chat.
- Open model/provider configuration.
- Sync or reconcile provider catalog if supported by current APIs.

## Responsive Behavior

- Desktop: multi-column card gallery.
- Tablet: two-column gallery.
- Mobile: single-column cards with sticky filter access.

## Acceptance Criteria

- Model Square is reachable from top navigation.
- Users can identify whether a model is usable.
- Users can jump from a model to Chat.
- Users can jump from a model to configuration where relevant.

