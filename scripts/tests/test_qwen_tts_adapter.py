import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "qwen_tts_adapter.py"


class QwenTTSAdapterTests(unittest.TestCase):
    def run_adapter(self, *args: str, env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
        merged = os.environ.copy()
        if env:
            merged.update(env)
        return subprocess.run(
            ["python3", str(SCRIPT), *args],
            cwd=ROOT,
            env=merged,
            text=True,
            capture_output=True,
            check=False,
        )

    def test_voices_uses_default_catalog(self) -> None:
        proc = self.run_adapter("voices", "--model", "qwen-tts-latest")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        payload = json.loads(proc.stdout)
        self.assertIn("voices", payload)
        self.assertTrue(any(item["id"] == "Cherry:bright" for item in payload["voices"]))
        self.assertTrue(any(item["id"] == "Cherry:soft" for item in payload["voices"]))

    def test_voices_flattens_character_timbres(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            catalog = Path(tmp) / "voices.json"
            catalog.write_text(
                json.dumps(
                    {
                        "characters": [
                            {
                                "id": "heroine",
                                "display_name": "Heroine",
                                "voices": [
                                    {
                                        "id": "bright",
                                        "display_name": "Bright",
                                        "downloaded": True,
                                    },
                                    {
                                        "id": "soft",
                                        "display_name": "Soft",
                                        "downloaded": False,
                                    },
                                ],
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            proc = self.run_adapter(
                "voices",
                "--model",
                "qwen-tts-latest",
                env={"QWEN_TTS_VOICES_FILE": str(catalog)},
            )
            self.assertEqual(proc.returncode, 0, proc.stderr)
            payload = json.loads(proc.stdout)
            self.assertEqual(
                payload["voices"],
                [
                    {
                        "id": "heroine:bright",
                        "display_name": "Heroine / Bright",
                        "character": "heroine",
                        "character_display_name": "Heroine",
                        "timbre": "bright",
                        "timbre_display_name": "Bright",
                        "downloaded": True,
                    },
                    {
                        "id": "heroine:soft",
                        "display_name": "Heroine / Soft",
                        "character": "heroine",
                        "character_display_name": "Heroine",
                        "timbre": "soft",
                        "timbre_display_name": "Soft",
                        "downloaded": False,
                    },
                ],
            )

    def test_synthesize_invokes_external_command_template(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "speech.wav"
            input_path = Path(tmp) / "input.txt"
            input_path.write_text("hello", encoding="utf-8")

            proc = self.run_adapter(
                "synthesize",
                "--model",
                "qwen-tts-latest",
                "--voice",
                "Cherry",
                "--input-file",
                str(input_path),
                "--output-file",
                str(output_path),
                "--response-format",
                "wav",
                env={
                    "QWEN_TTS_SYNTH_COMMAND": "python3 -c \"from pathlib import Path; Path(r'{output_file}').write_bytes(b'RIFFadapter')\""
                },
            )
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual(output_path.read_bytes(), b"RIFFadapter")

    def test_synthesize_resolves_reference_audio_from_catalog(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            output_path = Path(tmp) / "speech.wav"
            input_path = Path(tmp) / "input.txt"
            ref_path = Path(tmp) / "refs" / "cherry_bright.wav"
            ref_path.parent.mkdir(parents=True, exist_ok=True)
            ref_path.write_bytes(b"REF")
            input_path.write_text("hello", encoding="utf-8")
            catalog = Path(tmp) / "voices.json"
            catalog.write_text(
                json.dumps(
                    {
                        "characters": [
                            {
                                "id": "Cherry",
                                "display_name": "Cherry",
                                "voices": [
                                    {
                                        "id": "bright",
                                        "display_name": "Bright",
                                        "downloaded": True,
                                        "reference_audio": "refs/cherry_bright.wav",
                                    }
                                ],
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            capture_path = Path(tmp) / "captured.txt"

            proc = self.run_adapter(
                "synthesize",
                "--model",
                "qwen-tts-latest",
                "--voice",
                "Cherry:bright",
                "--input-file",
                str(input_path),
                "--output-file",
                str(output_path),
                "--response-format",
                "wav",
                env={
                    "QWEN_TTS_VOICES_FILE": str(catalog),
                    "QWEN_TTS_SYNTH_COMMAND": (
                        "python3 -c \"from pathlib import Path; "
                        "Path(r'{output_file}').write_bytes(b'RIFFadapter'); "
                        "Path(r'"
                        + str(capture_path)
                        + "').write_text(r'{reference_audio}', encoding='utf-8')\""
                    ),
                },
            )
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual(output_path.read_bytes(), b"RIFFadapter")
            self.assertEqual(capture_path.read_text(encoding='utf-8'), str(ref_path.resolve()))


if __name__ == "__main__":
    unittest.main()
