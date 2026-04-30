package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FunASRASRPlugin struct{}

func (p *FunASRASRPlugin) TranscribeAudio(ctx context.Context, modelID string, data []byte, filename string, mimeType string, extraPayload map[string]any, cfg map[string]any) (map[string]any, error) {
	workDir, cleanup, err := funASRTempWorkDir(cfg)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	inputPath := filepath.Join(workDir, sanitizeAudioFilename(filename))
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("write funasr input: %w", err)
	}

	args := []string{
		"transcribe",
		"--model", modelID,
		"--input-file", inputPath,
	}
	addOptionalArg := func(flag string, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, flag, strings.TrimSpace(value))
		}
	}
	addOptionalArg("--device", readString(cfg, "device"))
	addOptionalArg("--vad-model", readString(cfg, "vad_model"))
	addOptionalArg("--punc-model", readString(cfg, "punc_model"))
	addOptionalArg("--spk-model", readString(cfg, "spk_model"))
	addOptionalArg("--batch-size-s", scalarString(cfg["batch_size_s"]))
	addOptionalArg("--language", scalarString(extraPayload["language"]))
	addOptionalArg("--mime-type", mimeType)

	stdout, stderr, err := p.run(ctx, cfg, args...)
	if err != nil {
		return nil, wrapFunASRCLIError("transcribe", err, stderr)
	}
	out, err := decodeFunASROutput(stdout)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(readString(out, "text")) == "" {
		return nil, fmt.Errorf("funasr transcribe returned empty text")
	}
	return out, nil
}

func (p *FunASRASRPlugin) TranslateAudio(_ context.Context, _ string, _ []byte, _ string, _ string, _ map[string]any, _ map[string]any) (map[string]any, error) {
	return nil, fmt.Errorf("asr plugin funasr does not support audio translations")
}

func (p *FunASRASRPlugin) run(ctx context.Context, cfg map[string]any, args ...string) ([]byte, string, error) {
	command := strings.TrimSpace(readString(cfg, "command"))
	if command == "" {
		return nil, "", fmt.Errorf("asr plugin funasr requires command")
	}
	baseArgs := readStringSlice(cfg, "args")
	cmdArgs := append(append([]string{}, baseArgs...), args...)
	execCtx := ctx
	if timeout := readTimeout(cfg, 120*time.Second); timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(execCtx, command, cmdArgs...)
	if dir := strings.TrimSpace(readString(cfg, "command_working_dir")); dir != "" {
		cmd.Dir = dir
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	stdout, err := cmd.Output()
	return stdout, stderr.String(), err
}

func funASRTempWorkDir(cfg map[string]any) (string, func(), error) {
	parent := strings.TrimSpace(readString(cfg, "working_dir"))
	if parent == "" {
		parent = os.TempDir()
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, fmt.Errorf("create funasr working_dir: %w", err)
	}
	dir, err := os.MkdirTemp(parent, "funasr-asr-*")
	if err != nil {
		return "", nil, fmt.Errorf("create funasr temp dir: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func decodeFunASROutput(stdout []byte) (map[string]any, error) {
	out := map[string]any{}
	if err := json.Unmarshal(stdout, &out); err == nil {
		if _, ok := out["text"].(string); ok {
			return out, nil
		}
		return nil, fmt.Errorf("decode funasr output: missing text field")
	}
	text := strings.TrimSpace(string(stdout))
	if text == "" {
		return nil, fmt.Errorf("decode funasr output: empty stdout")
	}
	return map[string]any{"text": text}, nil
}

func sanitizeAudioFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "audio.bin"
	}
	return name
}

func scalarString(raw any) string {
	if raw == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if text == "<nil>" {
		return ""
	}
	return text
}

func wrapFunASRCLIError(action string, err error, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("funasr %s failed: %s", action, detail)
}
