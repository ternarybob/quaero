# Frontend Skill for Quaero

**Prerequisite:** Read `.claude/skills/refactoring/SKILL.md` before any code changes.

## Project Context
- **Templates:** Go html/template (server-side rendering)
- **Interactivity:** Alpine.js
- **Styling:** Bulma CSS
- **Real-time:** WebSockets for live updates

## Directory Structure
```
pages/
├── *.html              # Page templates
├── partials/           # Reusable components
│   ├── navbar.html
│   └── footer.html
└── static/
    ├── quaero.css      # Global styles
    └── common.js       # Common utilities
```

## Required Patterns

### Alpine.js Data Binding
```html
<div x-data="{ items: {{ .Items | json }}, loading: false }">
    <template x-for="item in items" :key="item.id">
        <div x-text="item.name"></div>
    </template>
</div>
```

### Bulma Components
```html
<!-- Cards -->
<div class="card">
    <div class="card-content">
        <p class="title is-5">Title</p>
    </div>
</div>

<!-- Buttons -->
<button class="button is-primary" @click="submit()">Save</button>

<!-- Forms -->
<div class="field">
    <label class="label">Name</label>
    <div class="control">
        <input class="input" type="text" x-model="name">
    </div>
</div>
```

### Template Composition
```html
{{template "navbar" .}}
<main class="section">
    {{template "content" .}}
</main>
{{template "footer" .}}
```

### WebSocket Events
```javascript
const ws = new WebSocket(`ws://${location.host}/ws`);
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    // Handle real-time updates
};
```

## Anti-Patterns (AUTO-FAIL)
```html
<!-- ❌ Inline styles -->
<div style="color: red;">

<!-- ❌ jQuery or other frameworks -->
<script src="jquery.js">

<!-- ❌ Client-side routing -->
<router-view>

<!-- ❌ React/Vue/SPA patterns -->
<div id="app"></div>
```
```javascript
// ❌ Direct DOM manipulation (use Alpine)
document.getElementById('x').innerHTML = y;

// ❌ Fetch without error handling
fetch('/api/data').then(r => r.json());
```

## Rules Summary

1. Server-side rendering - Go templates generate HTML
2. Alpine.js only - no other JS frameworks
3. Bulma CSS only - no custom CSS frameworks
4. WebSockets for real-time - not polling
5. Progressive enhancement - works without JS where possible