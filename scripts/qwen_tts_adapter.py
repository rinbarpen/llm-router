#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import shlex
import subprocess
import sys
from pathlib import Path
from typing import Any


ROOT_DIR = Path(__file__).resolve().parents[1]
DEFAULT_VOICE_FILE = ROOT_DIR / "scripts" / "qwen_tts_voices.json"


def _load_json(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def flatten_voice_catalog(payload: Any) -> dict[str, Any]:
    if isinstance(payload, list):
        return {"voices": payload}
    if isinstance(payload, dict):
        voices = payload.get("voices")
        if isinstance(voices, list):
            return {"voices": voices}
        characters = payload.get("characters")
        if isinstance(characters, list):
            flattened: list[dict[str, Any]] = []
            for character in characters:
                if not isinstance(character, dict):
                    continue
                character_id = str(character.get("id", "")).strip()
                if not character_id:
                    continue
                character_name = str(character.get("display_name", character_id)).strip() or character_id
                for voice in character.get("voices", []):
                    if not isinstance(voice, dict):
                        continue
                    timbre_id = str(voice.get("id", "")).strip()
                    if not timbre_id:
                        continue
                    timbre_name = str(voice.get("display_name", timbre_id)).strip() or timbre_id
                    item = dict(voice)
                    item["id"] = str(voice.get("voice_id", f"{character_id}:{timbre_id}")).strip()
                    item["display_name"] = str(
                        voice.get("full_display_name", f"{character_name} / {timbre_name}")
                    ).strip()
                    item["character"] = character_id
                    item["character_display_name"] = character_name
                    item["timbre"] = timbre_id
                    item["timbre_display_name"] = timbre_name
                    flattened.append(item)
            return {"voices": flattened}
    raise SystemExit("invalid voice catalog format")


def load_voice_catalog() -> dict[str, Any]:
    voice_file = os.environ.get("QWEN_TTS_VOICES_FILE", "").strip()
    path = Path(voice_file) if voice_file else DEFAULT_VOICE_FILE
    if not path.exists():
        return {"voices": []}
    payload = _load_json(path)
    try:
        return flatten_voice_catalog(payload)
    except SystemExit as exc:
        raise SystemExit(f"{exc}: {path}")


def resolve_voice_catalog_path() -> Path:
    voice_file = os.environ.get("QWEN_TTS_VOICES_FILE", "").strip()
    return Path(voice_file) if voice_file else DEFAULT_VOICE_FILE


def find_voice_metadata(voice_id: str) -> dict[str, Any] | None:
    payload = load_voice_catalog()
    for voice in payload.get("voices", []):
        if isinstance(voice, dict) and str(voice.get("id", "")).strip() == voice_id:
            return voice
    return None


def resolve_reference_audio_path(voice_id: str) -> str:
    voice = find_voice_metadata(voice_id)
    if not voice:
        return ""
    raw = str(voice.get("reference_audio", "")).strip()
    if not raw:
        return ""
    path = Path(raw)
    if not path.is_absolute():
        path = resolve_voice_catalog_path().resolve().parent / path
    return str(path.resolve())


def render_template(template: str, values: dict[str, str]) -> str:
    rendered = template
    for key, value in values.items():
        rendered = rendered.replace("{" + key + "}", value)
    return rendered


def run_shell_template(template: str, values: dict[str, str]) -> None:
    command = render_template(template, values)
    subprocess.run(command, shell=True, check=True, cwd=ROOT_DIR)


def cmd_voices(_: argparse.Namespace) -> int:
    list_command = os.environ.get("QWEN_TTS_LIST_COMMAND", "").strip()
    if list_command:
        completed = subprocess.run(
            render_template(list_command, {}),
            shell=True,
            check=True,
            cwd=ROOT_DIR,
            capture_output=True,
            text=True,
        )
        sys.stdout.write(completed.stdout)
        return 0
    payload = load_voice_catalog()
    json.dump(payload, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


def cmd_synthesize(args: argparse.Namespace) -> int:
    template = os.environ.get("QWEN_TTS_SYNTH_COMMAND", "").strip()
    if not template:
        raise SystemExit(
            "QWEN_TTS_SYNTH_COMMAND is not configured. "
            "Set it to a shell template using {model}, {voice}, {input_file}, {output_file}, {response_format}."
        )
    run_shell_template(
        template,
        {
            "model": args.model,
            "voice": args.voice,
            "input_file": args.input_file,
            "output_file": args.output_file,
            "response_format": args.response_format,
            "reference_audio": resolve_reference_audio_path(args.voice),
        },
    )
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="QwenTTS adapter for llm-router plugins")
    sub = parser.add_subparsers(dest="command", required=True)

    voices = sub.add_parser("voices", help="List available voices")
    voices.add_argument("--model", required=True)
    voices.set_defaults(func=cmd_voices)

    synthesize = sub.add_parser("synthesize", help="Synthesize speech")
    synthesize.add_argument("--model", required=True)
    synthesize.add_argument("--voice", required=True)
    synthesize.add_argument("--input-file", required=True)
    synthesize.add_argument("--output-file", required=True)
    synthesize.add_argument("--response-format", default="mp3")
    synthesize.add_argument("--speed")
    synthesize.set_defaults(func=cmd_synthesize)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return int(args.func(args))


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as exc:
        raise SystemExit(exc.returncode)
