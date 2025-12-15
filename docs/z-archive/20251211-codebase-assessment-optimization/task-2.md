# Task 2: Update Tree View to Light Theme

Depends: - | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent

Implements "Light Theme" requirement - black text on light gray background for better readability.

## Skill Patterns to Apply

- Alpine.js patterns for UI components
- CSS inline styles (existing pattern in codebase)
- Consistent color palette

## Do

1. Update tree view container colors in `pages/queue.html`:
   - Main container: `#1e1e1e` → `#f5f5f5` (light gray)
   - Header: `#252526` → `#e8e8e8` (slightly darker header)
   - Log area: `#1a1a1a` → `#fafafa` (very light gray)

2. Update text colors:
   - Primary text: `#d4d4d4` → `#333333` (dark gray)
   - Secondary text: `#888` → `#666` (medium gray)
   - Line numbers: `#555` → `#999` (light gray)

3. Update border colors:
   - Step borders: `#333` → `#ddd`
   - Log indent: `#333` → `#ccc`

4. Update input field styles:
   - Search input: dark theme → light theme
   - Button: dark theme → light theme

## Accept

- [ ] Tree view has light gray background (#f5f5f5)
- [ ] Text is dark (#333333) and easily readable
- [ ] Borders are light gray (#ddd)
- [ ] Search input has light theme styling
- [ ] Refresh button has light theme styling
- [ ] Visual appearance matches "light view" requirement
