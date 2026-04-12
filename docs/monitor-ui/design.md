# LLM Router Monitor UI Design

## Summary

The monitor UI should evolve from a single-purpose monitoring console into a unified LLM Router control deck. It must serve operators who manage runtime health, developers who need API and chat tooling, and administrators who manage model access and tokens.

The chosen product shape is a single application with two navigation layers:

- Top product navigation: `模型广场`, `Chat`, `Help`, `API Doc`.
- Left operations navigation: `仪表盘`, `令牌管理`, `日志信息`, `个人中心`.

The default landing page is `仪表盘`. It remains the operational home, while the top navigation keeps product-facing entry points visible at all times.

## Product Direction

The UI should feel like an editorial technology product rather than a generic admin dashboard. The visual language is warm, spacious, and product-led, with deeper embedded panels where operational density is needed.

The intended impression is:

- Product shell: readable, calm, editorial, documentation-friendly.
- Control surfaces: precise, high-contrast, data-dense when needed.
- Navigation: immediately separates product access from operational control.
- Experience: a user can start from system health, jump into Chat, inspect a model, read documentation, or manage tokens without changing applications.

## Information Architecture

Use stable page keys:

- `dashboard`
- `token-management`
- `logs`
- `profile`
- `model-square`
- `chat`
- `help`
- `api-doc`

Navigation groups:

- `productNavItems`: `model-square`, `chat`, `help`, `api-doc`.
- `opsNavItems`: `dashboard`, `token-management`, `logs`, `profile`.

Top navigation should be horizontal on desktop. On mobile, product navigation may wrap, collapse into a drawer, or become a segmented scroll rail, but it must remain distinct from operations navigation.

Left navigation should only contain operations pages. Do not put `模型广场`, `Chat`, `Help`, or `API Doc` in the left rail.

## Visual System

Use an `Editorial Tech` direction:

- Background: warm ivory or soft parchment, not pure white.
- Primary control color: deep navy or ink blue.
- Accent: muted brass/gold for active states, highlights, and high-value metrics.
- Supporting colors: slate blue, clay, soft gray, and restrained success/error colors.
- Typography: use a more editorial hierarchy than a standard dashboard. Page titles should feel intentional and spacious; table content should stay compact and readable.
- Cards: fewer default Ant Design cards; prefer custom surfaces with deliberate borders, soft shadows, and controlled depth.
- Motion: restrained page reveal, active navigation transitions, and hover polish. Avoid excessive animations in dense operational tables.

The design should keep Ant Design for behavior and accessibility, but override the default admin-dashboard appearance through shell layout, card styling, table framing, spacing, and typography.

## Layout System

Desktop layout:

- Sticky global header with brand, system status, theme toggle, export/action controls, and profile affordance.
- Horizontal product navigation below or inside the global header.
- Left operations rail for the four operations pages.
- Shared content region for all page bodies.
- Dashboard uses a high-level editorial hero plus embedded operational panels.

Mobile layout:

- Header remains compact and sticky.
- Operations rail becomes a drawer or bottom/stacked menu.
- Product navigation remains horizontally available as a scrollable rail or compact drawer section.
- Dense tables should prefer responsive cards or horizontal scroll with clear affordances.

## Data Integration Policy

Use mixed integration for v1:

- Real data: dashboard metrics, token/API key management, invocation logs, login logs, model/provider data, Chat.
- Static or local data: Help, API Doc, lightweight profile identity and preferences where backend data is unavailable.

Do not require backend schema changes for v1. If a page needs information not exposed by the backend, use static content, existing local storage, or derived summaries from existing frontend state.

## Component Strategy

Reuse existing components where behavior already exists:

- `ActivityDashboard`
- `APIKeyManagementPane`
- `InvocationList`
- `LoginRecordList`
- `ChatWorkbench`
- model/provider management components

Refactor at the shell and page-composition level before rewriting component internals. Existing tables, modals, forms, and API service contracts should keep their behavior unless a page-level design explicitly calls for a new interaction.

## Accessibility

The UI must support:

- Keyboard navigation across top nav, left nav, tables, forms, and modals.
- Visible focus states with sufficient contrast.
- Semantic headings per page.
- Clear active states for both nav layers.
- Meaningful button labels and aria labels where icon-only actions are used.
- Sufficient contrast in both light and dark surfaces.

## Acceptance Criteria

- The app has two visually distinct navigation layers: product top nav and operations side nav.
- Opening the monitor lands on `仪表盘`.
- Every page key renders a page.
- Existing API-backed monitor workflows still work.
- Help and API Doc are usable without backend dependencies.
- The UI no longer looks like a stock Ant Design admin dashboard.
- `npm run build` succeeds in `examples/monitor`.

