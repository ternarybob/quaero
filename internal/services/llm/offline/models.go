package offline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ternarybob/arbor"
)

// ModelInfo contains metadata about a model file
type ModelInfo struct {
	Path string
	Size int64
	Name string
}

// ModelManager handles model file verification and path management
type ModelManager struct {
	modelDir   string
	embedModel string
	chatModel  string
	logger     arbor.ILogger
}

// NewModelManager creates a new model manager instance
func NewModelManager(modelDir, embedModel, chatModel string, logger arbor.ILogger) *ModelManager {
	return &ModelManager{
		modelDir:   modelDir,
		embedModel: embedModel,
		chatModel:  chatModel,
		logger:     logger,
	}
}

// VerifyModels checks that both embedding and chat model files exist and are readable
func (m *ModelManager) VerifyModels() error {
	embedPath := m.GetEmbedModelPath()
	chatPath := m.GetChatModelPath()

	// Verify embedding model
	embedInfo, err := os.Stat(embedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("embedding model not found: %s", embedPath)
		}
		return fmt.Errorf("cannot access embedding model: %w", err)
	}
	if embedInfo.IsDir() {
		return fmt.Errorf("embedding model path is a directory: %s", embedPath)
	}
	if embedInfo.Size() == 0 {
		return fmt.Errorf("embedding model file is empty: %s", embedPath)
	}

	m.logger.Debug().
		Str("path", embedPath).
		Int64("size", embedInfo.Size()).
		Msg("Verified embedding model")

	// Verify chat model
	chatInfo, err := os.Stat(chatPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("chat model not found: %s", chatPath)
		}
		return fmt.Errorf("cannot access chat model: %w", err)
	}
	if chatInfo.IsDir() {
		return fmt.Errorf("chat model path is a directory: %s", chatPath)
	}
	if chatInfo.Size() == 0 {
		return fmt.Errorf("chat model file is empty: %s", chatPath)
	}

	m.logger.Debug().
		Str("path", chatPath).
		Int64("size", chatInfo.Size()).
		Msg("Verified chat model")

	m.logger.Info().Msg("All models verified successfully")
	return nil
}

// GetEmbedModelPath returns the full path to the embedding model file
func (m *ModelManager) GetEmbedModelPath() string {
	return filepath.Join(m.modelDir, m.embedModel)
}

// GetChatModelPath returns the full path to the chat model file
func (m *ModelManager) GetChatModelPath() string {
	return filepath.Join(m.modelDir, m.chatModel)
}

// GetModelInfo retrieves metadata about a model file
func (m *ModelManager) GetModelInfo(modelPath string) (ModelInfo, error) {
	info, err := os.Stat(modelPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ModelInfo{}, fmt.Errorf("model file not found: %s", modelPath)
		}
		return ModelInfo{}, fmt.Errorf("cannot access model file: %w", err)
	}

	if info.IsDir() {
		return ModelInfo{}, fmt.Errorf("path is a directory, not a model file: %s", modelPath)
	}

	modelInfo := ModelInfo{
		Path: modelPath,
		Size: info.Size(),
		Name: info.Name(),
	}

	m.logger.Debug().
		Str("path", modelPath).
		Str("name", modelInfo.Name).
		Int64("size", modelInfo.Size).
		Msg("Retrieved model info")

	return modelInfo, nil
}
