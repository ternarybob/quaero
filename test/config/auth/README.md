# API Keys Directory

This directory contains API key configuration files for Quaero. Files in this directory are automatically loaded on application startup.

## Usage

1. **Create a new API key file:**
   - Copy `example-api-keys.toml` to a new file (e.g., `google-places.toml`)
   - Edit the file and add your actual API key values

2. **File naming:**
   - Use descriptive names (e.g., `google-places.toml`, `gemini-llm.toml`)
   - Avoid spaces and special characters

3. **File format:**
   - Each file contains one API key credential
   - Use TOML format (similar to INI files)

## API Key File Structure

Each API key file must contain:

```toml
name = "unique-name-for-this-key"
api_key = "your-actual-api-key-here"
service_type = "service-identifier"
description = "Optional description"
```

## Required Fields

- **name**: Unique identifier for this API key (used in job definitions)
- **api_key**: The actual API key value
- **service_type**: Service identifier (e.g., "google-places", "gemini-llm", "agent")

## Optional Fields

- **description**: Human-readable description of this API key

## Example Files

### Google Places API
```toml
name = "google-places-key"
api_key = "AIzaSy..."
service_type = "google-places"
description = "Google Places API key for location search"
```

### Google Gemini LLM
```toml
name = "gemini-llm-key"
api_key = "AIza..."
service_type = "gemini-llm"
description = "Google Gemini API key for LLM features"
```

## Security Notes

- **Never commit API keys to version control**
- All `.toml` files in this directory are ignored by git (see `.gitignore`)
- API keys are automatically masked in logs and UI responses
- Use environment variables or secure credential management for production

## Service Types

Common service types:
- **google-places**: Google Places API for location search
- **gemini-llm**: Google Gemini API for LLM features
- **agent**: Google Agent API for web interaction
- **custom**: For custom services

## Usage in Job Definitions

Reference API keys in job definitions by name:

```toml
[steps.crawl]
action = "places_search"
config = {
    query = "restaurants near me",
    api_key = "google-places-key"  # Reference to name field
}
```

## Reloading

API key files are loaded on application startup. To reload:
1. Add/modify files in this directory
2. Restart the Quaero application

## Troubleshooting

- **API key not found**: Check that the file has valid TOML syntax
- **Service type mismatch**: Ensure service_type matches what the job expects
- **Permission denied**: Check file permissions (should be readable by Quaero user)
