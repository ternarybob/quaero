# Plan: Ollama References Cleanup

## Steps

1. **Clean up build script references**
   - Skill: @none
   - Files: scripts/build.ps1
   - User decision: no

2. **Update AGENTS.md documentation**
   - Skill: @none
   - Files: AGENTS.md
   - User decision: no

3. **Update README.md documentation**
   - Skill: @none
   - Files: README.md
   - User decision: no

4. **Update deployment configuration**
   - Skill: @none
   - Files: deployments/local/quaero.toml
   - User decision: no

5. **Update test configuration**
   - Skill: @none
   - Files: test/config/test-quaero.toml
   - User decision: no

6. **Update Chrome extension README**
   - Skill: @none
   - Files: cmd/quaero-chrome-extension/README.md
   - User decision: no

## Success Criteria
- All llama/ollama references removed from documentation
- Build script no longer manages llama-server processes
- Configuration files updated for Google ADK LLM service
- Documentation reflects cloud-based LLM architecture
- Test configurations work with graceful degradation
- Chrome extension documentation simplified
