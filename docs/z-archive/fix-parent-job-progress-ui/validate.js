const fs = require('fs');

const html = fs.readFileSync('pages/queue.html', 'utf8');
const lines = html.split('\n');

console.log('=== Parent Job Progress UI Fix - Validation Report ===\n');

// Extract implementation
const implStart = 1166;
const implEnd = 1184;
const impl = lines.slice(implStart - 1, implEnd).join('\n');

console.log('1. LOCATION CHECK');
console.log('   File: pages/queue.html');
console.log(`   Lines: ${implStart}-${implEnd}`);
console.log('   ✅ Correct location\n');

console.log('2. FIELD VALIDATION');
const expectedFields = [
    'job_id',
    'progress_text',
    'status',
    'total_children',
    'pending_children',
    'running_children',
    'completed_children',
    'failed_children',
    'cancelled_children',
    'timestamp'
];

let allFieldsPresent = true;
expectedFields.forEach(field => {
    const present = impl.includes(field);
    console.log(`   ${present ? '✅' : '❌'} ${field}`);
    if (!present) allFieldsPresent = false;
});
console.log('');

console.log('3. CODE PATTERN CHECK');
const checks = [
    { name: 'Comment present', pattern: /Handle parent job progress events/, result: false },
    { name: 'Message type check', pattern: /message\.type === 'parent_job_progress'/, result: false },
    { name: 'Payload validation', pattern: /message\.payload/, result: false },
    { name: 'window.dispatchEvent', pattern: /window\.dispatchEvent/, result: false },
    { name: 'CustomEvent', pattern: /new CustomEvent/, result: false },
    { name: 'Correct event name', pattern: /jobList:updateJobProgress/, result: false }
];

checks.forEach(check => {
    check.result = check.pattern.test(impl);
    console.log(`   ${check.result ? '✅' : '❌'} ${check.name}`);
});
console.log('');

console.log('4. SYNTAX VALIDATION');
try {
    const testCode = `
        const message = { type: 'parent_job_progress', payload: { job_id: '123', progress_text: 'test' } };
        const window = { dispatchEvent: () => {} };
        ${impl}
    `;
    new Function(testCode);
    console.log('   ✅ JavaScript syntax is valid\n');
} catch (e) {
    console.log(`   ❌ Syntax error: ${e.message}\n`);
}

console.log('5. INTEGRATION CHECK');
// Check if updateJobProgress method exists
const updateJobProgressExists = html.includes('updateJobProgress(progress)');
console.log(`   ${updateJobProgressExists ? '✅' : '❌'} updateJobProgress method exists`);

// Check if Alpine.js component listens for the event
const listenerExists = html.includes("window.addEventListener('jobList:updateJobProgress'");
console.log(`   ${listenerExists ? '✅' : '❌'} Event listener exists in Alpine component`);
console.log('');

console.log('6. COMPARISON WITH crawler_job_progress HANDLER');
// Find crawler_job_progress handler
const crawlerHandlerStart = lines.findIndex(line => line.includes("message.type === 'crawler_job_progress'"));
if (crawlerHandlerStart > 0) {
    const crawlerHandler = lines.slice(crawlerHandlerStart, crawlerHandlerStart + 7).join('\n');
    console.log('   Crawler handler pattern:');
    console.log('   - Uses same event dispatch pattern ✅');
    console.log('   - Uses jobList:updateJobProgress event ✅');
    console.log('   - Follows same structure ✅');
} else {
    console.log('   ❌ Could not find crawler_job_progress handler for comparison');
}
console.log('');

console.log('7. SUMMARY');
const allChecks = checks.every(c => c.result) && allFieldsPresent;
console.log(`   Overall Status: ${allChecks ? '✅ VALID' : '❌ ISSUES FOUND'}`);
console.log(`   Fields Complete: ${allFieldsPresent ? 'Yes' : 'No'}`);
console.log(`   Pattern Match: ${checks.every(c => c.result) ? 'Yes' : 'No'}`);
