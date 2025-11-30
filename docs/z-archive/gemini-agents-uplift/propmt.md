Problem:
The agent jobs appear to have different gemini configurations.
for 
bin\job-definitions\agent-document-generator.toml
bin\job-definitions\agent-web-enricher.toml
api key is listed

[steps.config]
# API key for LLM (resolved from KV store)
api_key = "{google_gemini_api_key}"

(And the jobs are not enabled)

bin\job-definitions\keyword-extractor-agent.toml
no api key listed, and the job is enabled.

actions:
An agent type job can either use the global api key or if one is included in [steps.config], then the job api key overrides. Note the model and other gemini settings operate the same.