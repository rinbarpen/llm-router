package plugins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQwenTTSPlugin_ListVoices(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "qwen-tts-adapter.sh")
	script := `#!/usr/bin/env bash
set -e
if [ "$1" = "voices" ]; then
  cat <<'JSON'
{"voices":[{"id":"xiaoyun","display_name":"Xiao Yun","downloaded":true},{"id":"xiaogang","display_name":"Xiao Gang","downloaded":false}]}
JSON
  exit 0
fi
echo "unexpected args: $*" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	plugin := &QwenTTSPlugin{}
	voices, err := plugin.ListVoices(context.Background(), "qwen-tts-latest", map[string]any{
		"command": scriptPath,
	})
	if err != nil {
		t.Fatalf("ListVoices() error = %v", err)
	}
	if len(voices) != 2 {
		t.Fatalf("voice count = %d, want 2", len(voices))
	}
	if voices[0].ID != "xiaoyun" || !voices[0].Downloaded {
		t.Fatalf("unexpected first voice: %+v", voices[0])
	}
	if voices[1].ID != "xiaogang" || voices[1].Downloaded {
		t.Fatalf("unexpected second voice: %+v", voices[1])
	}
}

func TestQwenTTSPlugin_SynthesizeSpeech(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "qwen-tts-adapter.sh")
	script := `#!/usr/bin/env bash
set -e
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
  printf 'RIFFfakewave' > "$out"
  exit 0
fi
echo "unexpected args: $*" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	plugin := &QwenTTSPlugin{}
	audio, contentType, err := plugin.SynthesizeSpeech(context.Background(), "qwen-tts-latest", map[string]any{
		"input":           "hello",
		"voice":           "xiaoyun",
		"response_format": "wav",
	}, map[string]any{
		"command": scriptPath,
	})
	if err != nil {
		t.Fatalf("SynthesizeSpeech() error = %v", err)
	}
	if contentType != "audio/wav" {
		t.Fatalf("contentType = %q, want audio/wav", contentType)
	}
	if !strings.HasPrefix(string(audio), "RIFF") {
		t.Fatalf("unexpected audio payload: %q", string(audio))
	}
}
