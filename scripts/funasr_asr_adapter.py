#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


def _compact_dict(values: dict[str, Any]) -> dict[str, Any]:
    return {key: value for key, value in values.items() if value not in (None, "")}


def _load_automodel() -> Any:
    try:
        from funasr import AutoModel
    except ImportError as exc:
        raise SystemExit(
            "funasr is not installed. Install FunASR in the adapter environment, "
            "for example: pip install funasr"
        ) from exc
    return AutoModel


def _extract_text(payload: Any) -> str:
    if isinstance(payload, str):
        return payload.strip()
    if isinstance(payload, dict):
        for key in ("text", "sentence", "transcript"):
            value = payload.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()
        result = payload.get("result")
        if result is not None:
            return _extract_text(result)
    if isinstance(payload, list):
        parts = [_extract_text(item) for item in payload]
        return " ".join(part for part in parts if part).strip()
    return ""


def _jsonable(payload: Any) -> Any:
    try:
        json.dumps(payload, ensure_ascii=False)
        return payload
    except TypeError:
        return repr(payload)


def cmd_transcribe(args: argparse.Namespace) -> int:
    input_path = Path(args.input_file)
    if not input_path.exists():
        raise SystemExit(f"input file does not exist: {input_path}")

    AutoModel = _load_automodel()
    model_kwargs = _compact_dict(
        {
            "model": args.model,
            "device": args.device,
            "vad_model": args.vad_model,
            "punc_model": args.punc_model,
            "spk_model": args.spk_model,
        }
    )
    model = AutoModel(**model_kwargs)

    generate_kwargs = _compact_dict(
        {
            "input": str(input_path),
            "batch_size_s": args.batch_size_s,
            "language": args.language,
        }
    )
    result = model.generate(**generate_kwargs)
    text = _extract_text(result)
    if not text:
        raise SystemExit("funasr returned no transcribed text")

    json.dump({"text": text, "raw": _jsonable(result)}, sys.stdout, ensure_ascii=False)
    sys.stdout.write("\n")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="FunASR adapter for llm-router ASR plugins")
    sub = parser.add_subparsers(dest="command", required=True)

    transcribe = sub.add_parser("transcribe", help="Transcribe an audio file")
    transcribe.add_argument("--model", required=True)
    transcribe.add_argument("--input-file", required=True)
    transcribe.add_argument("--device")
    transcribe.add_argument("--vad-model")
    transcribe.add_argument("--punc-model")
    transcribe.add_argument("--spk-model")
    transcribe.add_argument("--batch-size-s")
    transcribe.add_argument("--language")
    transcribe.add_argument("--mime-type")
    transcribe.set_defaults(func=cmd_transcribe)

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    return int(args.func(args))


if __name__ == "__main__":
    raise SystemExit(main())
