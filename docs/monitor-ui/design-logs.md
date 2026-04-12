# Logs Design

## Purpose

`日志信息` consolidates operational history into one place. It should bring together invocation history and login/access records so users can inspect router behavior and access events from a single page.

## Primary Users

- Operators debugging failed calls.
- Administrators auditing access events.
- Developers inspecting request/response details.

## Layout

Use a logs workspace layout:

- Page header with quick filters and export affordance.
- Segmented control or tabs for `调用日志` and `登录记录`.
- Main log table/list.
- Detail drawer for selected invocation or event.
- Optional compact summary strip for failures, total calls, and auth events in the selected time window.

The page can use deeper navy panels inside the warm product shell to signal operational depth.

## Key Components

- Log type switcher.
- Date/time range filters.
- Status filters.
- Provider/model filters where available.
- Invocation log table.
- Login record table.
- Invocation detail drawer/modal.
- Export actions where already supported.

## Data Sources

Use existing components and services:

- `InvocationList`
- `LoginRecordList`
- monitor export APIs where currently available

## Empty, Loading, And Error States

- Loading: table-level loading state.
- Empty: state no matching records for selected filters.
- Error: show retry action and preserve filters.

## Primary Interactions

- Switch between invocation logs and login records.
- Filter by status, auth method, provider/model, and time range where supported.
- Open invocation detail.
- Export logs through existing monitor export actions.

## Responsive Behavior

- Desktop: table-first layout with detail drawer.
- Mobile: filters stack above logs; tables may scroll horizontally.

## Acceptance Criteria

- Invocation history and login records are both reachable from `日志信息`.
- Existing invocation detail inspection remains available.
- Existing login record loading remains available.
- Filters do not reset unexpectedly when switching tabs unless technically unavoidable.

