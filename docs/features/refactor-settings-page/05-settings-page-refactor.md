I have created the following plan after thorough exploration and analysis of the codebase. Follow the below plan verbatim. Trust the files and references. Do not re-verify what's written in the plan. Explore only when absolutely necessary. First implement all the proposed file changes and then I'll review all the changes together at the end.

### Observations

**Current State:**
- Settings page uses Font Awesome icons (`fas fa-key`, `fas fa-lock`, etc.) instead of Spectre icons
- Custom accordion structure with `class="accordion-checkbox"` instead of just `hidden` attribute
- Custom loading spinner using Font Awesome (`fas fa-spinner fa-pulse fa-2x`)
- Extensive custom CSS in `quaero.css` (lines 1667-1746) with transitions, hover effects, and max-height animations
- Alpine.js component and backend routes are correctly implemented

**Target State:**
- Use Spectre's `icon icon-arrow-right` for all accordion headers (consistent across all sections)
- Simplify HTML to match Spectre's exact pattern: `<input type="checkbox" hidden>` (no extra class)
- Replace Font Awesome loading spinner with Spectre's `<div class="loading loading-lg"></div>`
- Reduce custom CSS to absolute minimum: only icon rotation for visual feedback
- Maintain all existing functionality (URL state, dynamic loading, multiple expansions)

**Key Insight:** The `search.html` page already demonstrates correct Spectre accordion usage with `<details class="accordion">` and `icon icon-arrow-right`, providing a reference pattern.

### Approach

Refine the settings accordion to use Spectre CSS native patterns with minimal custom CSS. Replace Font Awesome icons with Spectre icons (`icon icon-arrow-right`), update HTML structure to match Spectre's exact accordion pattern, replace custom loading spinners with Spectre's `loading` class, and add only essential CSS for icon rotation. The Alpine.js component and backend routes are already correctly implemented and require no changes.

### Reasoning

Read the current `settings.html` implementation showing Font Awesome icons and custom accordion structure. Examined `quaero.css` lines 1667-1746 revealing extensive custom accordion styling. Verified Spectre CSS is loaded via `head.html` (lines 8-10) including `spectre-icons.min.css`. Reviewed `search.html` showing correct Spectre accordion usage with `<details>` and `icon icon-arrow-right`. Confirmed via web search that Spectre accordion uses `<input type="checkbox" hidden>` + `<label class="accordion-header">` + `<i class="icon icon-arrow-right">` pattern, and loading spinners use `<div class="loading loading-lg"></div>`. The Alpine.js `settingsAccordion` component in `common.js` is already correctly implemented with URL state management.

## Proposed File Changes

### pages\settings.html(MODIFY)

References: 

- pages\search.html
- pages\partials\head.html

Update the accordion HTML structure (lines 22-114) to match Spectre CSS patterns exactly:

**Wrapper (line 22):**
- Change `<section x-data="settingsAccordion">` to `<div x-data="settingsAccordion" class="accordion">`
- Add the `accordion` class to the wrapper as per Spectre documentation

**Each Accordion Item (5 sections: auth-apikeys, auth-cookies, config, danger, status):**

1. **Input checkbox:**
   - Remove `class="accordion-checkbox"` attribute
   - Keep only `type="checkbox"`, `id`, `name="accordion-checkbox"`, `hidden`, and `@change` attributes
   - Example: `<input type="checkbox" id="accordion-auth-apikeys" name="accordion-checkbox" hidden @change="loadContent(...)">`

2. **Label (accordion header):**
   - Keep `class="accordion-header"` and `for` attribute
   - Replace Font Awesome icon with Spectre icon: `<i class="icon icon-arrow-right mr-1"></i>`
   - Remove the `<span>` wrapper around section title text - place text directly after icon
   - Example: `<label class="accordion-header" for="accordion-auth-apikeys"><i class="icon icon-arrow-right mr-1"></i>API Keys</label>`

3. **Accordion body:**
   - Keep `class="accordion-body"` on the wrapper div
   - Replace Font Awesome loading spinner (lines 32-36, 50-54, 68-72, 86-90, 104-108) with Spectre loading:
     - Change from: `<div x-show="loading['section']" class="text-center p-5"><span class="icon"><i class="fas fa-spinner fa-pulse fa-2x"></i></span><p class="mt-3">Loading...</p></div>`
     - Change to: `<div x-show="loading['section']" class="text-center" style="padding: 2rem;"><div class="loading loading-lg"></div><p style="margin-top: 1rem;">Loading...</p></div>`
   - Keep the content div with `x-show` and `x-html` attributes unchanged

**Section Order (already correct, verify alphabetical):**
1. API Keys (auth-apikeys)
2. Authentication (auth-cookies)
3. Configuration (config)
4. Danger Zone (danger)
5. Service Status (status)

**Closing tag (line 114):**
- Change `</section>` to `</div>` to match the wrapper change

**Rationale:** This brings the HTML into exact alignment with Spectre CSS accordion patterns as documented and demonstrated in `search.html`. The `icon icon-arrow-right` provides consistent iconography, and Spectre's `loading` class is the framework's native spinner pattern.

### pages\static\quaero.css(MODIFY)

References: 

- pages\settings.html(MODIFY)

Replace the extensive custom accordion styles (lines 1667-1746) with minimal CSS for icon rotation only:

**Remove (lines 1667-1721):**
- All custom `.accordion-item`, `.accordion-checkbox`, `.accordion-header`, `.accordion-body` styling
- Custom transitions, hover effects, max-height animations, border-radius adjustments
- These are now handled by Spectre CSS natively

**Add minimal icon rotation CSS (insert at line 1667):**
```css
/* Accordion Icon Rotation - Minimal CSS for functionality */
.accordion input[type="checkbox"]:checked + .accordion-header .icon {
    transform: rotate(90deg);
}

.accordion .accordion-header .icon {
    transition: transform 0.2s ease;
}
```

**Keep responsive adjustments (lines 1724-1745):**
- Retain the `@media (max-width: 768px)` block but remove the `.accordion-header` padding/font-size overrides (lines 1742-1745)
- Only keep job-card and terminal-job-context responsive rules

**Rationale:** Spectre CSS provides all the base accordion functionality (expand/collapse, styling, accessibility). The only missing piece is visual feedback for the arrow icon rotation, which requires 2 simple CSS rules. This achieves the "absolutely minimal" CSS requirement while maintaining usability. All aesthetic styling (colors, spacing, borders) defers to Spectre's defaults.