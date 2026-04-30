package plugins

import (
	"context"
	"strings"
	"testing"
)

func TestFunASRASRPluginTranslateUnsupported(t *testing.T) {
	plugin := &FunASRASRPlugin{}
	_, err := plugin.TranslateAudio(context.Background(), "paraformer-zh", []byte("audio"), "sample.wav", "audio/wav", nil, nil)
	if err == nil {
		t.Fatalf("TranslateAudio() error = nil, want unsupported error")
	}
	if !strings.Contains(err.Error(), "does not support audio translations") {
		t.Fatalf("TranslateAudio() error = %q", err)
	}
}
