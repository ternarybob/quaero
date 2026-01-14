# Frontend Skill

**Prerequisite:** Read `.codebuff/skills/refactoring/SKILL.md` first.

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
└── static/
    ├── *.css           # Global styles
    └── *.js            # Common utilities
```

## Required Patterns

### Alpine.js Data Binding
```html
<!-- Initialize Alpine.js data from Go template -->
<div x-data="{ items: {{ .Items | json }}, loading: false }">
    <template x-for="item in items" :key="item.id">
        <div x-text="item.name"></div>
    </template>
</div>

<!-- Event handling -->
<button @click="handleClick()" :disabled="loading">
    <span x-show="!loading">Submit</span>
    <span x-show="loading">Loading...</span>
</button>
```

### Bulma Components
```html
<!-- Card component -->
<div class="card">
    <div class="card-content">
        <p class="title is-5">Title</p>
        <p class="subtitle is-6">Subtitle</p>
    </div>
</div>

<!-- Button styles -->
<button class="button is-primary">Primary</button>
<button class="button is-danger is-outlined">Delete</button>

<!-- Form elements -->
<div class="field">
    <label class="label">Name</label>
    <div class="control">
        <input class="input" type="text" x-model="name">
    </div>
</div>
```

### Template Composition
```html
<!-- Base layout -->
{{template "head" .}}
{{template "navbar" .}}
<main class="section">
    {{template "content" .}}
</main>
{{template "footer" .}}

<!-- Define partial -->
{{define "content"}}
<div class="container">
    <!-- Page content -->
</div>
{{end}}
```

### WebSocket Events
```javascript
// Connect to WebSocket
const ws = new WebSocket(`ws://${location.host}/ws`);

// Handle messages
ws.onmessage = (event) => {
    const data = JSON.parse(event.data);
    // Update Alpine.js state
    Alpine.store('app').handleMessage(data);
};

// Handle reconnection
ws.onclose = () => {
    setTimeout(() => connectWebSocket(), 3000);
};
```

### Alpine.js Store (for shared state)
```javascript
// Define store
Alpine.store('app', {
    notifications: [],
    
    addNotification(msg) {
        this.notifications.push(msg);
        setTimeout(() => this.notifications.shift(), 5000);
    },
    
    handleMessage(data) {
        // Handle WebSocket message
    }
});

// Use in template
<div x-data x-show="$store.app.notifications.length > 0">
    <template x-for="n in $store.app.notifications">
        <div class="notification" x-text="n"></div>
    </template>
</div>
```

## Anti-Patterns (AUTO-FAIL)

```html
<!-- ❌ Inline styles -->
<div style="color: red; margin: 10px;">

<!-- ❌ jQuery -->
<script src="jquery.js"></script>
$('#element').click(...);

<!-- ❌ React/Vue/Angular -->
<div id="app"></div>
<script src="react.js"></script>

<!-- ❌ Dead template blocks -->
{{/* Old unused template - REMOVE! */}}
{{define "old_partial"}}...{{end}}
```

```javascript
// ❌ Direct DOM manipulation (use Alpine.js)
document.getElementById('x').innerHTML = y;
document.querySelector('.btn').addEventListener(...);

// ❌ Global variables
window.myData = {...};

// ❌ Unused functions
function oldHelper() { }  // REMOVE!

// ❌ Polling (use WebSockets)
setInterval(() => fetch('/api/status'), 5000);
```

## Rules Summary

1. **Server-side rendering** - Go templates generate HTML
2. **Alpine.js only** - No jQuery, React, Vue, or other JS frameworks
3. **Bulma CSS only** - No Bootstrap, Tailwind, or custom CSS frameworks
4. **WebSockets for real-time** - Not polling
5. **Progressive enhancement** - Works without JS where possible
6. **Remove dead templates/JS** - Clean up unused code
7. **No inline styles** - Use Bulma classes
8. **Alpine.js for state** - Not direct DOM manipulation

## Validation Checklist

- [ ] Uses Bulma CSS classes (no custom CSS framework)
- [ ] Uses Alpine.js for interactivity (no jQuery/React/Vue)
- [ ] WebSockets for real-time updates (no polling)
- [ ] No inline styles
- [ ] No dead template blocks
- [ ] No unused JavaScript functions
- [ ] Template composition via partials
- [ ] Proper Go template syntax
