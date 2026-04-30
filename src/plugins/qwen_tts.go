package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type QwenTTSPlugin struct{}

type qwenVoiceState struct {
	mu          sync.Mutex
	downloading bool
}

var qwenVoiceStates sync.Map

func (p *QwenTTSPlugin) ListVoices(ctx context.Context, modelID string, cfg map[string]any) ([]TTSVoice, error) {
	stdout, stderr, err := p.run(ctx, cfg, "voices", "--model", modelID)
	if err != nil {
		return nil, wrapCLIError("list voices", err, stderr)
	}
	voices, err := decodeVoices(stdout)
	if err != nil {
		return nil, err
	}
	for i := range voices {
		voices[i].Downloading = qwenVoiceDownloading(modelID, voices[i].ID)
	}
	return voices, nil
}

func (p *QwenTTSPlugin) SynthesizeSpeech(ctx context.Context, modelID string, payload map[string]any, cfg map[string]any) ([]byte, string, error) {
	input := strings.TrimSpace(readString(payload, "input"))
	if input == "" {
		return nil, "", fmt.Errorf("input is required")
	}
	voice := strings.TrimSpace(readString(payload, "voice"))
	if voice == "" {
		return nil, "", fmt.Errorf("voice is required")
	}
	responseFormat := strings.ToLower(strings.TrimSpace(readStringDefault(payload, "response_format", "mp3")))
	if responseFormat == "" {
		responseFormat = "mp3"
	}

	needsDownload := false
	if voices, err := p.ListVoices(ctx, modelID, cfg); err == nil {
		for _, item := range voices {
			if item.ID == voice {
				needsDownload = !item.Downloaded
				break
			}
		}
	}

	runSynthesis := func() ([]byte, string, error) {
		workDir, cleanup, err := qwenTempWorkDir(cfg)
		if err != nil {
			return nil, "", err
		}
		defer cleanup()

		inputPath := filepath.Join(workDir, "input.txt")
		outputPath := filepath.Join(workDir, "speech."+responseFormat)
		if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
			return nil, "", fmt.Errorf("write qwen tts input: %w", err)
		}
		args := []string{
			"synthesize",
			"--model", modelID,
			"--voice", voice,
			"--input-file", inputPath,
			"--output-file", outputPath,
			"--response-format", responseFormat,
		}
		if speed := strings.TrimSpace(fmt.Sprintf("%v", payload["speed"])); speed != "" && speed != "<nil>" {
			args = append(args, "--speed", speed)
		}
		_, stderr, err := p.run(ctx, cfg, args...)
		if err != nil {
			return nil, "", wrapCLIError("synthesize speech", err, stderr)
		}
		audio, err := os.ReadFile(outputPath)
		if err != nil {
			return nil, "", fmt.Errorf("read qwen tts output: %w", err)
		}
		return audio, qwenContentType(responseFormat), nil
	}

	if !needsDownload {
		return runSynthesis()
	}

	state := qwenVoiceStateFor(modelID, voice)
	state.mu.Lock()
	state.downloading = true
	defer func() {
		state.downloading = false
		state.mu.Unlock()
	}()
	return runSynthesis()
}

func (p *QwenTTSPlugin) run(ctx context.Context, cfg map[string]any, args ...string) ([]byte, string, error) {
	command := strings.TrimSpace(readString(cfg, "command"))
	if command == "" {
		return nil, "", fmt.Errorf("tts plugin qwen_tts requires command")
	}
	baseArgs := readStringSlice(cfg, "args")
	cmdArgs := append(append([]string{}, baseArgs...), args...)
	execCtx := ctx
	if timeout := readTimeout(cfg, 60*time.Second); timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(execCtx, command, cmdArgs...)
	if dir := strings.TrimSpace(readString(cfg, "working_dir")); dir != "" {
		cmd.Dir = dir
	}
	stderr := &strings.Builder{}
	cmd.Stderr = stderr
	stdout, err := cmd.Output()
	return stdout, stderr.String(), err
}

func qwenTempWorkDir(cfg map[string]any) (string, func(), error) {
	parent := strings.TrimSpace(readString(cfg, "working_dir"))
	if parent == "" {
		parent = os.TempDir()
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", nil, fmt.Errorf("create qwen tts working_dir: %w", err)
	}
	dir, err := os.MkdirTemp(parent, "qwen-tts-*")
	if err != nil {
		return "", nil, fmt.Errorf("create qwen tts temp dir: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

func decodeVoices(stdout []byte) ([]TTSVoice, error) {
	type response struct {
		Voices []TTSVoice `json:"voices"`
	}
	var wrapped response
	if err := json.Unmarshal(stdout, &wrapped); err == nil && len(wrapped.Voices) > 0 {
		return wrapped.Voices, nil
	}
	var voices []TTSVoice
	if err := json.Unmarshal(stdout, &voices); err != nil {
		return nil, fmt.Errorf("decode qwen tts voices: %w", err)
	}
	return voices, nil
}

func qwenVoiceStateFor(modelID string, voice string) *qwenVoiceState {
	key := modelID + "::" + voice
	state, _ := qwenVoiceStates.LoadOrStore(key, &qwenVoiceState{})
	return state.(*qwenVoiceState)
}

func qwenVoiceDownloading(modelID string, voice string) bool {
	key := modelID + "::" + voice
	state, ok := qwenVoiceStates.Load(key)
	if !ok {
		return false
	}
	return state.(*qwenVoiceState).downloading
}

func qwenContentType(responseFormat string) string {
	switch strings.ToLower(strings.TrimSpace(responseFormat)) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "ogg":
		return "audio/ogg"
	default:
		return "audio/mpeg"
	}
}

func wrapCLIError(action string, err error, stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("qwen_tts %s failed: %s", action, detail)
}
