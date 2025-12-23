package actions

import (
	"context"
	"testing"

	"github.com/ternarybob/arbor"
	"github.com/ternarybob/quaero/internal/interfaces"
	"github.com/ternarybob/quaero/internal/models"
)

// MockLLMService is a mock implementation of LLMService interface for testing
type MockLLMService struct {
	response  string
	err       error
	callCount int
}

func NewMockLLMService(response string, err error) *MockLLMService {
	return &MockLLMService{
		response: response,
		err:      err,
	}
}

func (m *MockLLMService) Chat(ctx context.Context, messages []interfaces.Message) (string, error) {
	m.callCount++
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *MockLLMService) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockLLMService) GetMode() interfaces.LLMMode {
	return interfaces.LLMModeCloud
}

func (m *MockLLMService) Close() error {
	return nil
}

// MockDocumentStorage is a mock implementation of DocumentStorage for testing
type MockDocumentStorage struct {
	documents map[string]*models.Document
}

func NewMockDocumentStorage() *MockDocumentStorage {
	return &MockDocumentStorage{
		documents: make(map[string]*models.Document),
	}
}

func (m *MockDocumentStorage) SaveDocument(doc *models.Document) error {
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) SaveDocuments(docs []*models.Document) error {
	for _, doc := range docs {
		m.documents[doc.ID] = doc
	}
	return nil
}

func (m *MockDocumentStorage) GetDocument(id string) (*models.Document, error) {
	return m.documents[id], nil
}

func (m *MockDocumentStorage) GetDocumentBySource(sourceType, sourceID string) (*models.Document, error) {
	for _, doc := range m.documents {
		if doc.SourceType == sourceType && doc.SourceID == sourceID {
			return doc, nil
		}
	}
	return nil, nil
}

func (m *MockDocumentStorage) UpdateDocument(doc *models.Document) error {
	m.documents[doc.ID] = doc
	return nil
}

func (m *MockDocumentStorage) DeleteDocument(id string) error {
	delete(m.documents, id)
	return nil
}

func (m *MockDocumentStorage) FullTextSearch(query string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) SearchByIdentifier(identifier string, excludeSources []string, limit int) ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) ListDocuments(opts *interfaces.ListOptions) ([]*models.Document, error) {
	docs := make([]*models.Document, 0, len(m.documents))
	for _, doc := range m.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

func (m *MockDocumentStorage) GetDocumentsBySource(sourceType string) ([]*models.Document, error) {
	docs := make([]*models.Document, 0)
	for _, doc := range m.documents {
		if doc.SourceType == sourceType {
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func (m *MockDocumentStorage) CountDocuments() (int, error) {
	return len(m.documents), nil
}

func (m *MockDocumentStorage) CountDocumentsBySource(sourceType string) (int, error) {
	count := 0
	for _, doc := range m.documents {
		if doc.SourceType == sourceType {
			count++
		}
	}
	return count, nil
}

func (m *MockDocumentStorage) GetStats() (*models.DocumentStats, error) {
	return &models.DocumentStats{}, nil
}

func (m *MockDocumentStorage) GetAllTags() ([]string, error) {
	return []string{}, nil
}

func (m *MockDocumentStorage) SetForceSyncPending(id string, pending bool) error {
	return nil
}

func (m *MockDocumentStorage) GetDocumentsForceSync() ([]*models.Document, error) {
	return []*models.Document{}, nil
}

func (m *MockDocumentStorage) ClearAll() error {
	m.documents = make(map[string]*models.Document)
	return nil
}

func (m *MockDocumentStorage) RebuildFTS5Index() error {
	return nil
}

func TestAnalyzeBuildSystemAction_IsBuildFile(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	tests := []struct {
		path     string
		expected bool
	}{
		{"Makefile", true},
		{"makefile", true},
		{"Makefile.am", true},
		{"GNUmakefile", true},
		{"rules.mk", true},
		{"CMakeLists.txt", true},
		{"FindPackage.cmake", true},
		{"project.vcxproj", true},
		{"project.vcxproj.filters", true},
		{"configure", true},
		{"configure.ac", true},
		{"solution.sln", true},
		{"/path/to/Makefile", true},
		{"/path/to/CMakeLists.txt", true},
		{"main.cpp", false},
		{"README.md", false},
		{"test.py", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := action.IsBuildFile(tt.path); got != tt.expected {
				t.Errorf("IsBuildFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestAnalyzeBuildSystemAction_ExtractMakefileTargets(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Extract simple targets", func(t *testing.T) {
		content := `
all: main.o utils.o
	gcc -o app main.o utils.o

main.o: main.c
	gcc -c main.c

clean:
	rm -f *.o app
`
		targets := action.ExtractMakefileTargets(content)

		if len(targets) < 3 {
			t.Errorf("Expected at least 3 targets, got %d: %v", len(targets), targets)
		}

		hasAll := false
		hasClean := false
		for _, target := range targets {
			if target == "all" {
				hasAll = true
			}
			if target == "clean" {
				hasClean = true
			}
		}

		if !hasAll {
			t.Error("Expected 'all' target")
		}
		if !hasClean {
			t.Error("Expected 'clean' target")
		}
	})

	t.Run("Skip variable targets", func(t *testing.T) {
		content := `
$(TARGET): main.o
	gcc -o $@ $<

%.o: %.c
	gcc -c $<
`
		targets := action.ExtractMakefileTargets(content)

		// Should skip $(TARGET) and %.o as they contain $ or %
		for _, target := range targets {
			if target == "$(TARGET)" || target == "%.o" {
				t.Errorf("Should not extract variable target: %s", target)
			}
		}
	})

	t.Run("Empty Makefile", func(t *testing.T) {
		targets := action.ExtractMakefileTargets("")

		if len(targets) != 0 {
			t.Errorf("Expected 0 targets, got %d", len(targets))
		}
	})
}

func TestAnalyzeBuildSystemAction_ExtractCMakeTargets(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Extract executables and libraries", func(t *testing.T) {
		content := `
add_executable(myapp main.cpp utils.cpp)
add_library(mylib STATIC lib.cpp)
add_library(mydll SHARED dll.cpp)
`
		targets := action.ExtractCMakeTargets(content)

		if len(targets) != 3 {
			t.Errorf("Expected 3 targets, got %d: %v", len(targets), targets)
		}

		hasMyApp := false
		hasMyLib := false
		for _, target := range targets {
			if target == "myapp" {
				hasMyApp = true
			}
			if target == "mylib" {
				hasMyLib = true
			}
		}

		if !hasMyApp {
			t.Error("Expected 'myapp' executable")
		}
		if !hasMyLib {
			t.Error("Expected 'mylib' library")
		}
	})

	t.Run("Empty CMakeLists.txt", func(t *testing.T) {
		targets := action.ExtractCMakeTargets("")

		if len(targets) != 0 {
			t.Errorf("Expected 0 targets, got %d", len(targets))
		}
	})
}

func TestAnalyzeBuildSystemAction_ExtractCompilerFlags(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Extract various compiler flags", func(t *testing.T) {
		content := `
CFLAGS = -DDEBUG -I/usr/include -L/usr/lib -O2
gcc -DVERSION=1.0 -I./headers -o app main.c
`
		flags := action.ExtractCompilerFlags(content)

		if len(flags) < 5 {
			t.Errorf("Expected at least 5 flags, got %d: %v", len(flags), flags)
		}

		hasDebug := false
		hasInclude := false
		hasLib := false
		hasOptimization := false

		for _, flag := range flags {
			if flag == "-DDEBUG" {
				hasDebug = true
			}
			if flag == "-I/usr/include" || flag == "-I./headers" {
				hasInclude = true
			}
			if flag == "-L/usr/lib" {
				hasLib = true
			}
			if flag == "-O2" {
				hasOptimization = true
			}
		}

		if !hasDebug {
			t.Error("Expected -DDEBUG flag")
		}
		if !hasInclude {
			t.Error("Expected -I flag")
		}
		if !hasLib {
			t.Error("Expected -L flag")
		}
		if !hasOptimization {
			t.Error("Expected -O flag")
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		flags := action.ExtractCompilerFlags("")

		if len(flags) != 0 {
			t.Errorf("Expected 0 flags, got %d", len(flags))
		}
	})
}

func TestAnalyzeBuildSystemAction_ExtractLinkedLibraries(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Extract libraries from -l flags", func(t *testing.T) {
		content := `
gcc -o app main.o -lpthread -lm -lssl
target_link_libraries(myapp pthread ssl crypto)
`
		libraries := action.ExtractLinkedLibraries(content)

		if len(libraries) < 5 {
			t.Errorf("Expected at least 5 libraries, got %d: %v", len(libraries), libraries)
		}

		hasPthread := false
		hasMath := false
		hasSSL := false

		for _, lib := range libraries {
			if lib == "pthread" {
				hasPthread = true
			}
			if lib == "m" {
				hasMath = true
			}
			if lib == "ssl" {
				hasSSL = true
			}
		}

		if !hasPthread {
			t.Error("Expected pthread library")
		}
		if !hasMath {
			t.Error("Expected m (math) library")
		}
		if !hasSSL {
			t.Error("Expected ssl library")
		}
	})

	t.Run("Skip CMake keywords", func(t *testing.T) {
		content := `
target_link_libraries(myapp PUBLIC pthread PRIVATE ssl INTERFACE crypto)
`
		libraries := action.ExtractLinkedLibraries(content)

		for _, lib := range libraries {
			if lib == "PUBLIC" || lib == "PRIVATE" || lib == "INTERFACE" {
				t.Errorf("Should not extract CMake keyword as library: %s", lib)
			}
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		libraries := action.ExtractLinkedLibraries("")

		if len(libraries) != 0 {
			t.Errorf("Expected 0 libraries, got %d", len(libraries))
		}
	})
}

func TestAnalyzeBuildSystemAction_ExtractObjectFiles(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Extract .o and .obj files", func(t *testing.T) {
		content := `
app: main.o utils.o helper.obj
	gcc -o app main.o utils.o helper.obj
`
		objFiles := action.ExtractObjectFiles(content)

		if len(objFiles) < 3 {
			t.Errorf("Expected at least 3 object files, got %d: %v", len(objFiles), objFiles)
		}

		hasMainO := false
		hasHelperObj := false

		for _, obj := range objFiles {
			if obj == "main.o" {
				hasMainO = true
			}
			if obj == "helper.obj" {
				hasHelperObj = true
			}
		}

		if !hasMainO {
			t.Error("Expected main.o")
		}
		if !hasHelperObj {
			t.Error("Expected helper.obj")
		}
	})

	t.Run("Empty content", func(t *testing.T) {
		objFiles := action.ExtractObjectFiles("")

		if len(objFiles) != 0 {
			t.Errorf("Expected 0 object files, got %d", len(objFiles))
		}
	})
}

func TestAnalyzeBuildSystemAction_AnalyzeWithLLM(t *testing.T) {
	logger := arbor.NewLogger()

	t.Run("Successful LLM analysis", func(t *testing.T) {
		jsonResponse := `{
  "build_targets": ["app", "tests"],
  "compiler_flags": ["-O2", "-Wall"],
  "linked_libraries": ["pthread", "m"],
  "build_deps": ["main.o", "utils.o"],
  "toolchain": "gcc",
  "notes": "Standard makefile setup"
}`

		mockLLM := NewMockLLMService(jsonResponse, nil)
		action := NewAnalyzeBuildSystemAction(nil, mockLLM, logger)

		content := "all: app\napp: main.o\n\tgcc -o app main.o"
		result, err := action.AnalyzeWithLLM(context.Background(), content, "Makefile")

		if err != nil {
			t.Fatalf("AnalyzeWithLLM failed: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if targets, ok := result["build_targets"].([]interface{}); ok {
			if len(targets) != 2 {
				t.Errorf("Expected 2 build targets, got %d", len(targets))
			}
		} else {
			t.Error("Expected build_targets in result")
		}

		if mockLLM.callCount != 1 {
			t.Errorf("Expected 1 LLM call, got %d", mockLLM.callCount)
		}
	})

	t.Run("LLM returns markdown code blocks", func(t *testing.T) {
		jsonResponse := "```json\n{\"build_targets\": [\"app\"]}\n```"

		mockLLM := NewMockLLMService(jsonResponse, nil)
		action := NewAnalyzeBuildSystemAction(nil, mockLLM, logger)

		content := "all: app"
		result, err := action.AnalyzeWithLLM(context.Background(), content, "Makefile")

		if err != nil {
			t.Fatalf("AnalyzeWithLLM should handle markdown blocks: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}
	})

	t.Run("LLM returns malformed JSON", func(t *testing.T) {
		malformedJSON := `{build_targets: [app]}`

		mockLLM := NewMockLLMService(malformedJSON, nil)
		action := NewAnalyzeBuildSystemAction(nil, mockLLM, logger)

		content := "all: app"
		_, err := action.AnalyzeWithLLM(context.Background(), content, "Makefile")

		if err == nil {
			t.Error("Expected error for malformed JSON")
		}
	})

	t.Run("Truncate long content", func(t *testing.T) {
		jsonResponse := `{"build_targets": ["app"]}`
		mockLLM := NewMockLLMService(jsonResponse, nil)
		action := NewAnalyzeBuildSystemAction(nil, mockLLM, logger)

		// Create content longer than maxContentLength (4000)
		longContent := ""
		for i := 0; i < 5000; i++ {
			longContent += "a"
		}

		result, err := action.AnalyzeWithLLM(context.Background(), longContent, "Makefile")

		if err != nil {
			t.Fatalf("AnalyzeWithLLM failed: %v", err)
		}

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		// Should handle truncation without error
	})
}

func TestAnalyzeBuildSystemAction_Execute(t *testing.T) {
	logger := arbor.NewLogger()
	storage := NewMockDocumentStorage()

	t.Run("Analyze Makefile", func(t *testing.T) {
		action := NewAnalyzeBuildSystemAction(storage, nil, logger)

		doc := &models.Document{
			ID:  "test-makefile",
			URL: "/path/to/Makefile",
			ContentMarkdown: `
all: app
app: main.o utils.o
	gcc -o app main.o utils.o -lpthread -lm
main.o: main.c
	gcc -c main.c -DDEBUG
`,
			Metadata: make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata")
		}

		if devopsData == nil {
			t.Fatal("Expected non-nil devops metadata")
		}
	})

	t.Run("Skip non-build file", func(t *testing.T) {
		action := NewAnalyzeBuildSystemAction(storage, nil, logger)

		doc := &models.Document{
			ID:              "test-cpp",
			URL:             "/path/to/main.cpp",
			ContentMarkdown: "int main() { return 0; }",
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		// Should skip processing
	})

	t.Run("Analyze CMakeLists.txt", func(t *testing.T) {
		action := NewAnalyzeBuildSystemAction(storage, nil, logger)

		doc := &models.Document{
			ID:  "test-cmake",
			URL: "/path/to/CMakeLists.txt",
			ContentMarkdown: `
cmake_minimum_required(VERSION 3.10)
project(MyApp)
add_executable(myapp main.cpp)
target_link_libraries(myapp pthread ssl)
`,
			Metadata: make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		devopsData, ok := doc.Metadata["devops"]
		if !ok {
			t.Fatal("Expected devops metadata")
		}

		if devopsData == nil {
			t.Fatal("Expected non-nil devops metadata")
		}
	})

	t.Run("With LLM service", func(t *testing.T) {
		jsonResponse := `{"build_targets": ["app"], "compiler_flags": ["-O2"], "linked_libraries": ["pthread"]}`
		mockLLM := NewMockLLMService(jsonResponse, nil)
		action := NewAnalyzeBuildSystemAction(storage, mockLLM, logger)

		// Content must be > 100 chars for LLM to be called
		longContent := `# Makefile for testing LLM integration
all: app
app: main.o utils.o helper.o
	gcc -o app main.o utils.o helper.o -lpthread -lm

main.o: main.c config.h
	gcc -c main.c -DDEBUG -I./include

utils.o: utils.c utils.h
	gcc -c utils.c

clean:
	rm -f *.o app
`
		doc := &models.Document{
			ID:              "test-with-llm",
			URL:             "/path/to/Makefile",
			ContentMarkdown: longContent,
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if mockLLM.callCount != 1 {
			t.Errorf("Expected LLM to be called once, got %d calls", mockLLM.callCount)
		}
	})
}

func TestAnalyzeBuildSystemAction_EdgeCases(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Complex multi-target Makefile", func(t *testing.T) {
		content := `
.PHONY: all clean install test

all: app1 app2 app3

app1: src/app1.o lib/utils.o
	gcc -o bin/app1 src/app1.o lib/utils.o

app2: src/app2.o lib/utils.o
	gcc -o bin/app2 src/app2.o lib/utils.o

clean:
	rm -rf bin/*.o bin/app*

install: all
	cp bin/app* /usr/local/bin/

test: all
	./bin/app1 --test
`
		targets := action.ExtractMakefileTargets(content)

		// Should extract multiple targets including .PHONY targets
		if len(targets) < 5 {
			t.Errorf("Expected at least 5 targets from complex Makefile, got %d: %v", len(targets), targets)
		}
	})

	t.Run("CMake with subdirectories", func(t *testing.T) {
		content := `
add_subdirectory(src)
add_subdirectory(tests)
add_executable(main main.cpp)
add_library(utils STATIC utils.cpp)
`
		targets := action.ExtractCMakeTargets(content)

		// Should extract main and utils, but not subdirectories
		if len(targets) != 2 {
			t.Errorf("Expected 2 targets, got %d: %v", len(targets), targets)
		}
	})

	t.Run("Visual Studio project file", func(t *testing.T) {
		content := `
<Project>
  <PropertyGroup>
    <ProjectName>MyApp</ProjectName>
    <ConfigurationType>Application</ConfigurationType>
  </PropertyGroup>
</Project>
`
		storage := NewMockDocumentStorage()
		action := NewAnalyzeBuildSystemAction(storage, nil, logger)

		doc := &models.Document{
			ID:              "test-vcxproj",
			URL:             "/path/to/project.vcxproj",
			ContentMarkdown: content,
			Metadata:        make(map[string]interface{}),
		}

		err := action.Execute(context.Background(), doc, false)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
	})

	t.Run("uniqueStrings helper", func(t *testing.T) {
		input := []string{"a", "b", "a", "c", "b", "", "d", ""}
		result := uniqueStrings(input)

		// Should remove duplicates and empty strings
		if len(result) != 4 {
			t.Errorf("Expected 4 unique non-empty strings, got %d: %v", len(result), result)
		}

		// Check no duplicates
		seen := make(map[string]bool)
		for _, s := range result {
			if seen[s] {
				t.Errorf("Found duplicate string: %s", s)
			}
			seen[s] = true
			if s == "" {
				t.Error("Empty string should be filtered out")
			}
		}
	})
}

func TestAnalyzeBuildSystemAction_MergeLLMResults(t *testing.T) {
	logger := arbor.NewLogger()
	action := NewAnalyzeBuildSystemAction(nil, nil, logger)

	t.Run("Merge LLM results with existing metadata", func(t *testing.T) {
		devopsData := &models.DevOpsMetadata{
			BuildTargets:    []string{"existing_target"},
			CompilerFlags:   []string{"-O2"},
			LinkedLibraries: []string{"existing_lib"},
		}

		llmData := map[string]interface{}{
			"build_targets":    []interface{}{"new_target"},
			"compiler_flags":   []interface{}{"-Wall"},
			"linked_libraries": []interface{}{"new_lib"},
			"build_deps":       []interface{}{"main.o"},
			"toolchain":        "gcc",
			"notes":            "Test notes",
		}

		action.mergeLLMResults(devopsData, llmData)

		// Should merge, not replace
		if len(devopsData.BuildTargets) != 2 {
			t.Errorf("Expected 2 build targets after merge, got %d", len(devopsData.BuildTargets))
		}
		if len(devopsData.CompilerFlags) != 2 {
			t.Errorf("Expected 2 compiler flags after merge, got %d", len(devopsData.CompilerFlags))
		}
		if len(devopsData.LinkedLibraries) != 2 {
			t.Errorf("Expected 2 linked libraries after merge, got %d", len(devopsData.LinkedLibraries))
		}
		if len(devopsData.BuildDeps) != 1 {
			t.Errorf("Expected 1 build dep, got %d", len(devopsData.BuildDeps))
		}
	})

	t.Run("Handle empty LLM results", func(t *testing.T) {
		devopsData := &models.DevOpsMetadata{
			BuildTargets: []string{"target1"},
		}

		llmData := map[string]interface{}{}

		action.mergeLLMResults(devopsData, llmData)

		// Should preserve existing data
		if len(devopsData.BuildTargets) != 1 {
			t.Errorf("Expected 1 build target, got %d", len(devopsData.BuildTargets))
		}
	})

	t.Run("Handle invalid LLM result types", func(t *testing.T) {
		devopsData := &models.DevOpsMetadata{}

		llmData := map[string]interface{}{
			"build_targets":    "not an array",
			"compiler_flags":   123,
			"linked_libraries": true,
		}

		// Should not panic
		action.mergeLLMResults(devopsData, llmData)
	})
}
