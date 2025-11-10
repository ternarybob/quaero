# Documentation Cleanup Needed (Step 5)

## Files to Update

### CLAUDE.md
**Lines to review/remove:**
- Lines 137, 150-158: Event-driven architecture mentions embedding events
- Lines 180-202: "LLM Service Architecture" section (entire section)
- Lines 215: Remove `embedding, embedding_model` from schema description
- Lines 752-760: "Modifying LLM Behavior" section (entire section)
- Lines 809, 818-821: Embedding coordinator and force_embed_pending references
- Lines 827: Query embedding generation in RAG section

**Replacement text:**
- Update architecture diagram to remove LLM/Embedding services
- Update event list to remove EventEmbeddingTriggered
- Remove RAG implementation section or mark as "Future: API-based AI integration"

### README.md
**Search for and remove:**
- Ollama/llama references
- Embedding/RAG feature descriptions
- LLM configuration examples

### AGENTS.md
**Check for:**
- LLM service references
- Embedding/chat feature descriptions

## Steps 6-7 Status

### Step 6: Build Script
**Check:** `scripts/build.ps1` for llama-cli checks
**Status:** Likely no changes needed (build script doesn't check for llama binaries)

### Step 7: Server Configuration
**Status:** ✅ DONE - LlamaDir already removed from ServerConfig struct and env var handling

## Recommendation
Run full documentation cleanup after verifying Step 8 (testing) is successful. This ensures we don't document features that might need to be restored.
