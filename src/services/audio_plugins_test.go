package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTrySynthesizeWithPlugin_QwenTTS(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "qwen-tts-adapter.sh")
	script := `#!/usr/bin/env bash
set -e
if [ "$1" = "voices" ]; then
  cat <<'JSON'
{"voices":[{"id":"xiaoyun","display_name":"Xiao Yun","downloaded":true}]}
JSON
  exit 0
fi
if [ "$1" = "synthesize" ]; then
  out=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "--output-file" ]; then
      out="$2"
      shift 2
      continue
    fi
    shift
  done
  printf 'RIFFservice' > "$out"
  exit 0
fi
echo "unexpected args: $*" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write adapter script: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "router.toml")
	cfg := `
[plugins.tts.qwen_tts]
command = "` + scriptPath + `"
default_model = "qwen-tts-latest"
models = ["qwen-tts-latest"]
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write router.toml: %v", err)
	}
	t.Setenv("LLM_ROUTER_MODEL_CONFIG_FILE", cfgPath)

	audio, contentType, handled, err := trySynthesizeWithPlugin(context.Background(), map[string]any{
		"model":           "plugin:qwen_tts/qwen-tts-latest",
		"input":           "hello",
		"response_format": "wav",
		"role":            "xiaoyun",
	})
	if err != nil {
		t.Fatalf("trySynthesizeWithPlugin() error = %v", err)
	}
	if !handled {
		t.Fatalf("expected plugin path to be handled")
	}
	if contentType != "audio/wav" {
		t.Fatalf("contentType = %q, want audio/wav", contentType)
	}
	if string(audio) != "RIFFservice" {
		t.Fatalf("audio = %q, want RIFFservice", string(audio))
	}
}

func TestTryTranscribeWithPlugin_FunASR(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "funasr-asr-adapter.sh")
	capturePath := filepath.Join(tmpDir, "input.txt")
	script := `#!/usr/bin/env bash
set -e
if [ "$1" = "transcribe" ]; then
  input=""
  model=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --input-file)
        input="$2"
        shift 2
        ;;
      --model)
        model="$2"
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done
  test "$model" = "paraformer-zh"
  cat "$input" > "` + capturePath + `"
  printf '{"text":"hello from funasr"}\n'
  exit 0
fi
echo "unexpected args: $*" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write adapter script: %v", err)
	}

	cfgPath := filepath.Join(tmpDir, "router.toml")
	cfg := `
[plugins.asr.funasr]
command = "` + scriptPath + `"
default_model = "paraformer-zh"
models = ["paraformer-zh"]
working_dir = "` + tmpDir + `"
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write router.toml: %v", err)
	}
	t.Setenv("LLM_ROUTER_MODEL_CONFIG_FILE", cfgPath)

	out, handled, err := tryTranscribeWithPlugin(context.Background(), map[string]any{
		"model": "plugin:funasr/paraformer-zh",
	}, []byte("RIFFservice"), "sample.wav", "audio/wav", false)
	if err != nil {
		t.Fatalf("tryTranscribeWithPlugin() error = %v", err)
	}
	if !handled {
		t.Fatalf("expected plugin path to be handled")
	}
	if out["text"] != "hello from funasr" {
		t.Fatalf("text = %#v, want hello from funasr", out["text"])
	}
	if got, err := os.ReadFile(capturePath); err != nil || string(got) != "RIFFservice" {
		t.Fatalf("captured input = %q, err=%v", string(got), err)
	}
}

func TestNormalizeSpeechPayload_PrefersVoice(t *testing.T) {
	payload := normalizeSpeechPayload(map[string]any{
		"voice": "alloy",
		"role":  "xiaoyun",
	})
	if payload["voice"] != "alloy" {
		t.Fatalf("voice = %#v, want alloy", payload["voice"])
	}
	if _, ok := payload["role"]; ok {
		t.Fatalf("role should be removed after normalization")
	}
}
