import json
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "scripts" / "funasr_asr_adapter.py"


class FunASRASRAdapterTests(unittest.TestCase):
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

    def test_transcribe_invokes_funasr_automodel(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            audio_path = tmp_path / "sample.wav"
            audio_path.write_bytes(b"RIFFadapter")
            capture_path = tmp_path / "capture.json"
            fake_module = tmp_path / "funasr.py"
            fake_module.write_text(
                textwrap.dedent(
                    f"""
                    import json
                    from pathlib import Path

                    CAPTURE = Path({str(capture_path)!r})

                    class AutoModel:
                        def __init__(self, **kwargs):
                            self.kwargs = kwargs

                        def generate(self, **kwargs):
                            CAPTURE.write_text(
                                json.dumps({{"model_kwargs": self.kwargs, "generate_kwargs": kwargs}}),
                                encoding="utf-8",
                            )
                            return [{{"text": "hello from fake funasr"}}]
                    """
                ),
                encoding="utf-8",
            )

            proc = self.run_adapter(
                "transcribe",
                "--model",
                "paraformer-zh",
                "--input-file",
                str(audio_path),
                "--device",
                "cpu",
                "--vad-model",
                "fsmn-vad",
                "--punc-model",
                "ct-punc",
                "--batch-size-s",
                "300",
                "--language",
                "zh",
                env={"PYTHONPATH": tmp},
            )

            self.assertEqual(proc.returncode, 0, proc.stderr)
            payload = json.loads(proc.stdout)
            self.assertEqual(payload["text"], "hello from fake funasr")
            captured = json.loads(capture_path.read_text(encoding="utf-8"))
            self.assertEqual(
                captured["model_kwargs"],
                {
                    "model": "paraformer-zh",
                    "device": "cpu",
                    "vad_model": "fsmn-vad",
                    "punc_model": "ct-punc",
                },
            )
            self.assertEqual(
                captured["generate_kwargs"],
                {"input": str(audio_path), "batch_size_s": "300", "language": "zh"},
            )

    def test_transcribe_requires_text_result(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = Path(tmp)
            audio_path = tmp_path / "sample.wav"
            audio_path.write_bytes(b"RIFFadapter")
            fake_module = tmp_path / "funasr.py"
            fake_module.write_text(
                textwrap.dedent(
                    """
                    class AutoModel:
                        def __init__(self, **kwargs):
                            pass

                        def generate(self, **kwargs):
                            return [{"tokens": []}]
                    """
                ),
                encoding="utf-8",
            )

            proc = self.run_adapter(
                "transcribe",
                "--model",
                "paraformer-zh",
                "--input-file",
                str(audio_path),
                env={"PYTHONPATH": tmp},
            )

            self.assertNotEqual(proc.returncode, 0)
            self.assertIn("returned no transcribed text", proc.stderr)


if __name__ == "__main__":
    unittest.main()
