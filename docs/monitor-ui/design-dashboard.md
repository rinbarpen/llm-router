# Dashboard Design

## Purpose

`仪表盘` is the default operational home for LLM Router. It gives users an immediate view of router health, cost, token usage, request volume, errors, and recent activity.

## Primary Users

- Operators checking system health.
- Administrators monitoring spend and token consumption.
- Developers investigating recent routing behavior.

## Layout

Use an editorial overview layout rather than a simple grid of equal cards:

- Top hero summary with current system state, selected time range, and the most important metric.
- Secondary metric cards for spend, tokens, calls, and error rate.
- Trend section for time-series activity.
- Recent activity section for latest invocation records.
- Shortcut panel linking to `令牌管理`, `日志信息`, `模型广场`, and `Chat`.

The page should combine warm product-shell spacing with deeper operational panels for charts and recent events.

## Key Components

- Time range selector.
- Refresh action.
- Metric summary cards.
- Time-series chart.
- Recent invocation list.
- Shortcut/action cards.

## Data Sources

Use existing statistics, time-series, and invocation data through the current monitor/db service layer.

Expected existing sources include:

- Overall statistics.
- Time-series request/token/cost data.
- Recent invocation records.

## Empty, Loading, And Error States

- Loading: show skeleton cards or a contained spinner in each data region.
- Empty: show a clear message that no calls exist for the selected time range.
- Error: show a recoverable alert with retry action. Keep the page shell visible.

## Primary Interactions

- Change time range.
- Refresh metrics.
- Open recent invocation details.
- Jump to logs for deeper inspection.
- Jump to token management if usage or access issues are detected.

## Responsive Behavior

- Desktop: hero and metrics should use a multi-column composition.
- Tablet: metrics wrap to two columns.
- Mobile: hero, metrics, chart, and recent activity stack vertically.

## Acceptance Criteria

- Dashboard is the default page.
- Cost, token, call, and recent activity information are visible above the fold on desktop.
- Time range changes update the data.
- Recent invocation drill-down behavior remains available.
- Existing `ActivityDashboard` behavior is preserved or cleanly composed into the new page.

