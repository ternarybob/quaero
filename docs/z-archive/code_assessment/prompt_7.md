1. The events (sent to the client) need to be buffered, as they are limiting the speed of the service.
C:/Users/bobmc/Pictures/Screenshots/ksnip_20251209-163106.png

2. The step "classify_files" "LLM classification for ambiguous files (category=unknown only)" is classifing ALL files. This should not be the case, as the previous step should have classificied the majority.
C:/Users/bobmc/Pictures/Screenshots/ksnip_20251209-163548.png
bin\logs\quaero.2025-12-09T16-27-59.log
bin\job-definitions\codebase_assess.toml