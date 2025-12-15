# Task 5: Convert Scrollable Text Boxes to Divs

Depends: 2 | Critical: no | Model: sonnet | Skill: frontend

## Addresses User Intent

Implements "Div vs Scrollable" - use div elements for log display instead of scrollable text boxes.

## Skill Patterns to Apply

- CSS layout patterns
- Responsive design
- Natural height expansion

## Do

1. Identify scrollable containers in tree view:
   - Main tree container (`max-height: 400px; overflow-y: auto;`)
   - Log containers within steps

2. Remove inner scroll from log containers:
   - Remove `overflow-y: auto` from step log divs
   - Let log content expand naturally within step

3. Keep outer container scroll:
   - Maintain `max-height: 400px; overflow-y: auto` on main tree container
   - This allows overall tree to scroll while individual steps expand

4. Update log display:
   - Use plain `div` for each log line
   - Natural height based on content
   - Word-wrap for long log messages

5. Ensure proper styling:
   - Consistent padding/margin
   - Proper text wrapping
   - No horizontal overflow

## Accept

- [ ] Log lines use div elements (not pre/textarea)
- [ ] Step content expands naturally
- [ ] Only outer tree container has scroll
- [ ] Long log messages wrap properly
- [ ] No unwanted horizontal scrollbars
