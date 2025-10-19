I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

The service logs component is already well-structured with Alpine.js reactive data management. The \`serviceLogs\` component in \`pages/static/common.js\` already parses log levels and assigns CSS classes. The UI header in \`pages/partials/service-logs.html\` has a button group that can accommodate the new dropdown. No backend changes are required since log level information is already included in the WebSocket messages and API responses.

### Approach

Add a log level filter dropdown to the service logs component that allows users to filter logs by severity (All, Error, Warning, Info, Debug). The filter will be implemented in the existing Alpine.js \`serviceLogs\` component with localStorage persistence to remember user preferences across sessions. The dropdown will be styled consistently with existing UI buttons in the header section.

### Reasoning

I explored the repository structure, read the relevant files for the service logs feature (\`pages/partials/service-logs.html\`, \`pages/static/common.js\`, \`pages/static/quaero.css\`, and \`internal/handlers/websocket.go\`), and identified the existing implementation patterns for log management and UI components.

## Mermaid Diagram

sequenceDiagram
    participant User
    participant HTML as service-logs.html
    participant Alpine as serviceLogs Component
    participant Storage as localStorage
    participant Display as Terminal Display

    User->>HTML: Page loads
    HTML->>Alpine: init()
    Alpine->>Storage: Load saved filter preference
    Storage-->>Alpine: Return 'quaero_log_level_filter'
    Alpine->>Alpine: Set selectedLogLevel
    Alpine->>Alpine: Compute filteredLogs
    Alpine->>Display: Render filtered logs

    User->>HTML: Select log level from dropdown
    HTML->>Alpine: setLogLevel(level)
    Alpine->>Storage: Save preference to localStorage
    Alpine->>Alpine: Update selectedLogLevel
    Alpine->>Alpine: Recompute filteredLogs
    Alpine->>Display: Re-render with filtered logs

    Note over Alpine,Display: WebSocket receives new log
    Alpine->>Alpine: addLog(logData)
    Alpine->>Alpine: Recompute filteredLogs
    Alpine->>Display: Update display with filtered results

## Proposed File Changes

### pages\\partials\\service-logs.html(MODIFY)

References: 

- pages\\static\\common.js(MODIFY)

Add a \`<select>\` dropdown element in the header section between the title and the button group. The dropdown should include options for 'All', 'Error', 'Warning', 'Info', and 'Debug' log levels. Bind the dropdown to Alpine.js using \`x-model=\"selectedLogLevel\"\` and \`@change=\"setLogLevel(\$event.target.value)\"\`. Add appropriate styling classes (form-select, select-sm) to match the existing button sizes. Update the template loop to iterate over \`filteredLogs\` instead of \`logs\` to display only the filtered results.

### pages\\static\\common.js(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

In the \`serviceLogs\` Alpine.js component, add a new reactive property \`selectedLogLevel\` with default value 'all'. Add a computed property \`filteredLogs\` that returns filtered logs based on \`selectedLogLevel\` - when 'all' is selected, return all logs; otherwise filter by matching the log level (case-insensitive comparison). Add a \`setLogLevel(level)\` method that updates \`selectedLogLevel\` and persists the preference to localStorage using key 'quaero_log_level_filter'. In the \`init()\` method, load the saved filter preference from localStorage and apply it to \`selectedLogLevel\`. Ensure the filtering logic handles level name variations (e.g., 'WARN' vs 'WARNING', 'ERR' vs 'ERROR').

### pages\\static\\quaero.css(MODIFY)

References: 

- pages\\partials\\service-logs.html(MODIFY)

Add CSS rules for the log level filter dropdown to ensure it matches the existing UI style. Create a \`.log-level-filter\` class for the select element with appropriate sizing (height matching the btn-sm buttons, approximately 32px), border-radius matching \`var(--border-radius)\`, border color using \`var(--border-color)\`, and background color using \`var(--content-bg)\`. Add padding (0.375rem 0.75rem) and font-size (0.875rem) to match button styling. Include focus styles with border-color \`var(--color-primary)\` and box-shadow for accessibility. Add margin-right spacing (0.5rem) to separate it from the button group. Ensure the dropdown integrates visually with the existing header layout.