package offline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
)

func TestNewOfflineLLMService_MockMode(t *testing.T) {
	logger := arbor.NewLogger()

	// Create temporary model directory
	tmpDir := t.TempDir()

	// Create dummy model files
	embedModelPath := filepath.Join(tmpDir, "embed.gguf")
	chatModelPath := filepath.Join(tmpDir, "chat.gguf")

	if err := os.WriteFile(embedModelPath, []byte("dummy model"), 0644); err != nil {
		t.Fatalf("Failed to create dummy embed model: %v", err)
	}
	if err := os.WriteFile(chatModelPath, []byte("dummy model"), 0644); err != nil {
		t.Fatalf("Failed to create dummy chat model: %v", err)
	}

	// Create service (will fail to find llama-cli, but that's ok for mock testing)
	service := &OfflineLLMService{
		modelManager: NewModelManager(tmpDir, "embed.gguf", "chat.gguf", logger),
		contextSize:  2048,
		threadCount:  4,
		gpuLayers:    0,
		logger:       logger,
		llamaCLIPath: "/fake/path/llama-cli", // Won't be used in mock mode
		mockMode:     false,
	}

	// Enable mock mode
	service.SetMockMode(true)

	// Test Embed in mock mode
	ctx := context.Background()
	embedding, err := service.Embed(ctx, "test text")
	if err != nil {
		t.Fatalf("Embed failed in mock mode: %v", err)
	}

	if len(embedding) != 768 {
		t.Errorf("Expected 768-dimension embedding, got %d", len(embedding))
	}

	// Test Chat in mock mode
	messages := []interfaces.Message{
		{Role: "user", Content: "Hello"},
	}
	response, err := service.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat failed in mock mode: %v", err)
	}

	if response == "" {
		t.Error("Expected non-empty response in mock mode")
	}

	// Test GetMode
	if service.GetMode() != interfaces.LLMModeOffline {
		t.Errorf("Expected mode %s, got %s", interfaces.LLMModeOffline, service.GetMode())
	}

	// Test Close
	if err := service.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestModelManager_VerifyModels(t *testing.T) {
	logger := arbor.NewLogger()
	tmpDir := t.TempDir()

	// Test with missing models
	manager := NewModelManager(tmpDir, "missing.gguf", "also-missing.gguf", logger)
	err := manager.VerifyModels()
	if err == nil {
		t.Error("Expected error for missing models, got nil")
	}

	// Create model files
	embedPath := filepath.Join(tmpDir, "embed.gguf")
	chatPath := filepath.Join(tmpDir, "chat.gguf")

	if err := os.WriteFile(embedPath, []byte("test model data"), 0644); err != nil {
		t.Fatalf("Failed to create embed model: %v", err)
	}
	if err := os.WriteFile(chatPath, []byte("test model data"), 0644); err != nil {
		t.Fatalf("Failed to create chat model: %v", err)
	}

	// Test with valid models
	manager = NewModelManager(tmpDir, "embed.gguf", "chat.gguf", logger)
	err = manager.VerifyModels()
	if err != nil {
		t.Errorf("VerifyModels failed with valid models: %v", err)
	}

	// Test GetModelInfo
	info, err := manager.GetModelInfo(embedPath)
	if err != nil {
		t.Errorf("GetModelInfo failed: %v", err)
	}
	if info.Name != "embed.gguf" {
		t.Errorf("Expected name 'embed.gguf', got '%s'", info.Name)
	}
	if info.Size != 15 {
		t.Errorf("Expected size 15, got %d", info.Size)
	}
}

func TestFormatPrompt(t *testing.T) {
	logger := arbor.NewLogger()
	service := &OfflineLLMService{
		logger: logger,
	}

	tests := []struct {
		name     string
		messages []interfaces.Message
		want     string
	}{
		{
			name: "system and user",
			messages: []interfaces.Message{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Hello"},
			},
			want: "<|im_start|>system\nYou are helpful<|im_end|>\n<|im_start|>user\nHello<|im_end|>\n<|im_start|>assistant\n",
		},
		{
			name: "conversation with assistant",
			messages: []interfaces.Message{
				{Role: "user", Content: "Hi"},
				{Role: "assistant", Content: "Hello!"},
				{Role: "user", Content: "How are you?"},
			},
			want: "<|im_start|>user\nHi<|im_end|>\n<|im_start|>assistant\nHello!<|im_end|>\n<|im_start|>user\nHow are you?<|im_end|>\n<|im_start|>assistant\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.formatPrompt(tt.messages)
			if got != tt.want {
				t.Errorf("formatPrompt() mismatch\nGot:\n%s\nWant:\n%s", got, tt.want)
			}
		})
	}
}

func TestExtractResponse(t *testing.T) {
	logger := arbor.NewLogger()
	service := &OfflineLLMService{
		logger: logger,
	}

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "simple response",
			output: "This is the response",
			want:   "This is the response",
		},
		{
			name: "response with debug output",
			output: `llama_model_loader: loaded meta data
ggml_backend_metal_init: loaded Metal kernel
This is the actual response
llama_perf_context_print: tokens per second = 25.3`,
			want: "This is the actual response",
		},
		{
			name: "multiline response",
			output: `Here is a response
that spans multiple lines
with useful information`,
			want: "Here is a response\nthat spans multiple lines\nwith useful information",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.extractResponse(tt.output)
			if got != tt.want {
				t.Errorf("extractResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateMockEmbedding(t *testing.T) {
	logger := arbor.NewLogger()
	service := &OfflineLLMService{
		logger: logger,
	}

	// Test that same text produces same embedding
	text := "test text"
	embedding1 := service.generateMockEmbedding(text)
	embedding2 := service.generateMockEmbedding(text)

	if len(embedding1) != 768 {
		t.Errorf("Expected 768-dimension embedding, got %d", len(embedding1))
	}

	// Check deterministic
	for i := range embedding1 {
		if embedding1[i] != embedding2[i] {
			t.Error("Mock embeddings should be deterministic")
			break
		}
	}

	// Test that different text produces different embedding
	embedding3 := service.generateMockEmbedding("different text")
	same := true
	for i := range embedding1 {
		if embedding1[i] != embedding3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Different text should produce different embeddings")
	}
}
