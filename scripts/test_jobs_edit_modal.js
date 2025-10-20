const { chromium } = require('playwright');

async function testJobsEditModal() {
    const browser = await chromium.launch({ headless: false });
    const context = await browser.newContext();
    const page = await context.newPage();

    try {
        // Navigate to the jobs page
        console.log('Navigating to http://localhost:8085/jobs');
        await page.goto('http://localhost:8085/jobs');
        
        // Wait for the page to load
        await page.waitForLoadState('networkidle');
        
        // Wait for the jobs table to load
        console.log('Waiting for jobs table...');
        await page.waitForSelector('#default-jobs-table-body', { timeout: 10000 });
        
        // Wait a bit more for jobs to load
        await page.waitForTimeout(2000);
        
        // Look for edit buttons in the jobs panel
        console.log('Looking for edit buttons...');
        const editButtons = await page.locator('button[title="Edit Job"] i.fa-edit').count();
        console.log(`Found ${editButtons} edit buttons`);
        
        if (editButtons === 0) {
            console.log('No edit buttons found. Checking page content...');
            const pageContent = await page.content();
            console.log('Page title:', await page.title());
            
            // Check if jobs loaded
            const jobRows = await page.locator('#default-jobs-table-body tr').count();
            console.log(`Found ${jobRows} job rows`);
            
            return;
        }
        
        // Click the first edit button
        console.log('Clicking first edit button...');
        await page.locator('button[title="Edit Job"]').first().click();
        
        // Wait for modal to appear
        await page.waitForSelector('#edit-default-job-modal.active', { timeout: 5000 });
        console.log('Modal appeared');
        
        // Fill in description
        await page.fill('#edit-default-job-description', 'Test description update');
        
        // Click Update Job button
        console.log('Clicking Update Job button...');
        await page.click('button:has-text("Update Job")');
        
        // Wait to see what happens
        await page.waitForTimeout(3000);
        
        // Check if loading spinner is visible
        const loadingVisible = await page.isVisible('#edit-default-modal-loading:not(.hide)');
        console.log('Loading spinner visible:', loadingVisible);
        
        // Check if modal is still open
        const modalStillOpen = await page.isVisible('#edit-default-job-modal.active');
        console.log('Modal still open:', modalStillOpen);
        
        // Check for any JavaScript errors
        page.on('console', msg => {
            if (msg.type() === 'error') {
                console.log('Browser console error:', msg.text());
            }
        });
        
        // Wait a bit longer to see final state
        await page.waitForTimeout(2000);
        
    } catch (error) {
        console.error('Error during test:', error);
    } finally {
        await browser.close();
    }
}

testJobsEditModal();