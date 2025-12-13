// Partial loader for including HTML fragments
async function loadPartial(selector, partialPath) {
    try {
        const response = await fetch(partialPath);
        if (!response.ok) {
            console.error(`Failed to load partial: ${partialPath}`);
            return;
        }
        const html = await response.text();
        const element = document.querySelector(selector);
        if (element) {
            element.innerHTML = html;

            // Execute any scripts in the partial
            const scripts = element.querySelectorAll('script');
            scripts.forEach(script => {
                const newScript = document.createElement('script');
                if (script.src) {
                    newScript.src = script.src;
                } else {
                    newScript.textContent = script.textContent;
                }
                document.body.appendChild(newScript);
                script.remove();
            });
        }
    } catch (error) {
        console.error(`Error loading partial ${partialPath}:`, error);
    }
}

// Load all partials when DOM is ready
document.addEventListener('DOMContentLoaded', async () => {
    const includes = document.querySelectorAll('[data-include]');
    const loadPromises = [];
    
    includes.forEach(element => {
        const partialPath = element.getAttribute('data-include');
        const selector = `[data-include="${partialPath}"]`;
        loadPromises.push(loadPartial(selector, partialPath));
    });
    
    // Wait for all partials to load
    await Promise.all(loadPromises);
    
    console.log('[PartialLoader] All partials loaded');
});
