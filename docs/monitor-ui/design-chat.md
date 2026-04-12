# Chat Design

## Purpose

`Chat` is the product-facing workbench for testing LLM Router chat completions, streaming, tools, and multimodal workflows.

## Primary Users

- Developers testing models.
- Operators validating provider behavior.
- Admins checking routing outcomes.

## Layout

Use a focused workbench layout:

- Session list.
- Main message panel.
- Composer.
- Model/settings panel.
- Debug/trace panel when available.

The page should be visually cleaner and more spacious than a dense internal tool. It belongs in the top product navigation, not the left operations rail.

## Key Components

- Session list and session creation.
- Model selector.
- Prompt composer.
- Message rendering.
- Streaming response controls.
- Settings controls for temperature, max tokens, top-p, and stream mode.
- Tool/skills JSON fields where currently supported.
- Debug traces/tool calls panel.
- Multimodal controls if currently available in `ChatWorkbench`.

## Data Sources

Use the existing Chat API and local session storage:

- Active model list.
- Chat completions.
- Streaming chat completions.
- Local Chat session history.
- Existing multimodal APIs where already integrated.

## Empty, Loading, And Error States

- Empty session: provide prompt suggestions and model selection CTA.
- Loading models: disable send until a model is available.
- Streaming: show stop control.
- Error: append recoverable error state to the conversation or show message notification.

## Primary Interactions

- Create, rename, and delete sessions.
- Select model and template.
- Send message.
- Stop streaming.
- Add images/audio where currently supported.
- Inspect raw/debug output.

## Responsive Behavior

- Desktop: three-pane workbench if space allows.
- Tablet: session list collapses.
- Mobile: session list and settings become drawers or stacked panels.

## Acceptance Criteria

- Existing Chat behavior remains functional.
- Chat is reachable from top navigation.
- Session persistence remains local and stable.
- Streaming and stop behavior remain available.
- The UI is less visually cluttered than the current embedded workbench.

