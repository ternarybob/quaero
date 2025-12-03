# Feature: Confluence/Jira Session Recording Capture

- Slug: confluence-jira-capture | Type: feature | Date: 2025-12-03
- Request: "Capture Confluence/Jira articles for AI querying. Headless browsing blocked, need JS-rendered pages with images. Options: session recording, extension-controlled navigation, or full-page image capture."
- Prior: none (builds on existing extension HTML capture from commit d3a80e8)

## User Intent

Enable capturing content from Confluence and Jira (JavaScript-rendered, authentication-required enterprise wiki pages) where:
1. Headless browser access is blocked by the platform
2. User authentication/cookies are required
3. JavaScript rendering is mandatory for content visibility
4. Embedded images need to be captured (not just links)

The captured content should be queryable/summarizable by AI for knowledge management purposes.

## Context

The screenshot shows a Confluence workspace ("LIMS Engineering Uplift") with a complex page tree navigation and rich content including:
- Hierarchical document structure (nested pages/articles)
- Multiple content types (Discovery, Analysis, Implementation docs)
- DevOps-related technical documentation
- Internal enterprise content requiring authentication

## Constraints

**Not Possible:**
- Using Atlassian APIs (restricted/unavailable)
- Direct browsing without cookies/authentication
- Headless Chrome (blocked by Confluence)

**Requirements:**
- JavaScript-rendered page capture
- Image capture (actual image data, not URLs)
- Authentication preservation via existing cookie forwarding

## Evaluated Options

| Option | Description | Feasibility | Complexity |
|--------|-------------|-------------|------------|
| 1 | Single page capture as full-page image, AI OCR conversion | High | Medium |
| 2 | Session recording - user enables "record" mode, each page auto-sent | High | Medium |
| 3 | Extension controls browser navigation | Medium | High |
| 4 | Backend service controls browser | Low | Very High |
| 5 | Different Go browser library | Low | Very High |

## Selected Approach: Option 2 - Session Recording Mode

**Rationale:**
- Leverages existing extension architecture (popup, sidepanel, content script)
- User maintains natural browsing flow
- Works with any JS-rendered page
- Reuses existing cookie/auth capture
- Can capture images inline (convert to base64 data URIs)
- Minimal friction - toggle on, browse normally, content captured automatically

## Success Criteria

- [ ] Extension has "Record Session" toggle in sidepanel UI
- [ ] When recording enabled, each page navigation triggers automatic capture
- [ ] Captured content includes full rendered HTML (post-JavaScript execution)
- [ ] Images in the page are converted to embedded data URIs (base64)
- [ ] Backend receives and stores captured pages with metadata
- [ ] Recording state persists across page navigations
- [ ] User can see capture history/status in sidepanel
- [ ] Works with Confluence pages (verified with screenshot context)
