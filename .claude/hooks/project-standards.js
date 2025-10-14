#!/usr/bin/env node
/**
 * Consolidated Claude Hooks for Quaero Project
 * Handles: prompt reminders, validation, duplicate detection, indexing
 */

const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');

// ============================================================================
// Configuration
// ============================================================================

const CONFIG = {
  MAX_FILE_LINES: 500,
  MAX_FUNC_LINES: 80,
  FORBIDDEN_PATTERNS: ['TODO', 'FIXME'],
  INDEX_PATH: path.join(process.cwd(), '.claude', 'go-function-index.json'),
  REQUIRED_LIBS: {
    logging: 'github.com/ternarybob/arbor',
    banner: 'github.com/ternarybob/banner',
    config: 'github.com/pelletier/go-toml/v2'
  }
};

// ============================================================================
// Utility Functions
// ============================================================================

function readGoFile(filePath) {
  try {
    return fs.readFileSync(filePath, 'utf-8');
  } catch {
    return null;
  }
}

function countLines(content) {
  return content.split('\n').length;
}

function extractFunctions(content) {
  const functions = [];
  const lines = content.split('\n');

  lines.forEach((line, idx) => {
    const match = line.match(/^func\s+(?:\(([^)]+)\)\s+)?([A-Z]\w+)\s*\(/);
    if (match) {
      const [, receiver, name] = match;
      functions.push({
        name,
        line: idx + 1,
        isReceiver: !!receiver,
        signature: line.trim()
      });
    }
  });

  return functions;
}

function validateDirectory(filePath) {
  const errors = [];

  if (filePath.includes('/internal/common/') || filePath.includes('\\internal\\common\\')) {
    const content = readGoFile(filePath);
    if (content && /func\s+\([^)]+\)/.test(content)) {
      errors.push('❌ BLOCKED: Receiver methods not allowed in internal/common/');
      errors.push('   Move to internal/services/ for stateful services');
    }
  }

  return errors;
}

function validateLogging(content) {
  const errors = [];
  const forbiddenLogging = [
    /fmt\.Println/,
    /fmt\.Printf/,
    /log\.Println/,
    /log\.Printf/
  ];

  forbiddenLogging.forEach(pattern => {
    if (pattern.test(content)) {
      errors.push(`❌ BLOCKED: Use arbor logger instead of ${pattern.source}`);
    }
  });

  return errors;
}

function validateErrorHandling(content) {
  const errors = [];

  if (/\b_\s*=\s*\w+\(.*\)\s*$/.test(content)) {
    errors.push('❌ BLOCKED: Ignored errors detected (using _ = )');
    errors.push('   All errors must be handled properly');
  }

  return errors;
}

// ============================================================================
// Function Index Management
// ============================================================================

function loadFunctionIndex() {
  try {
    const data = fs.readFileSync(CONFIG.INDEX_PATH, 'utf-8');
    return JSON.parse(data);
  } catch {
    return { lastUpdated: null, functions: [] };
  }
}

function saveFunctionIndex(index) {
  try {
    fs.mkdirSync(path.dirname(CONFIG.INDEX_PATH), { recursive: true });
    fs.writeFileSync(CONFIG.INDEX_PATH, JSON.stringify(index, null, 2));
  } catch (error) {
    console.error('⚠️  Failed to save function index:', error.message);
  }
}

function rebuildFunctionIndex() {
  const index = {
    lastUpdated: new Date().toISOString(),
    functions: []
  };

  try {
    // Use platform-appropriate command
    const isWindows = process.platform === 'win32';
    const command = isWindows
      ? 'dir /s /b *.go 2>nul'
      : 'find . -name "*.go" -type f ! -path "./vendor/*" 2>/dev/null';

    const goFiles = execSync(command, {
      encoding: 'utf-8',
      maxBuffer: 10 * 1024 * 1024
    }).split('\n').filter(Boolean);

    goFiles.forEach(file => {
      const content = readGoFile(file);
      if (!content) return;

      const functions = extractFunctions(content);
      functions.forEach(func => {
        index.functions.push({
          ...func,
          file: file.replace(/^\.\//, '').replace(/\\/g, '/')
        });
      });
    });

    saveFunctionIndex(index);
    console.log(`✅ Indexed ${index.functions.length} Go functions`);
  } catch (error) {
    console.error('⚠️  Function indexing failed:', error.message);
  }
}

function checkForDuplicates(filePath, content) {
  const index = loadFunctionIndex();
  const newFunctions = extractFunctions(content);
  const errors = [];

  // Normalize file path for comparison
  const normalizedPath = filePath.replace(/^\.\//, '').replace(/\\/g, '/');

  newFunctions.forEach(newFunc => {
    const duplicates = index.functions.filter(existing =>
      existing.name === newFunc.name &&
      existing.file !== normalizedPath
    );

    if (duplicates.length > 0) {
      errors.push(`❌ BLOCKED: Duplicate function '${newFunc.name}'`);
      duplicates.forEach(dup => {
        errors.push(`   Already exists: ${dup.file}:${dup.line}`);
      });
    }
  });

  return errors;
}

// ============================================================================
// Validation Functions
// ============================================================================

function validateGoFile(filePath, content) {
  const errors = [];

  // File length check
  const lineCount = countLines(content);
  if (lineCount > CONFIG.MAX_FILE_LINES) {
    errors.push(`⚠️  WARNING: File has ${lineCount} lines (max ${CONFIG.MAX_FILE_LINES})`);
    errors.push('   Consider splitting into smaller modules');
  }

  // Function length check (simplified - just warn about very long functions)
  const funcBlocks = content.match(/func\s+.*?\n(?:.*?\n)*?^\}/gm);
  if (funcBlocks) {
    funcBlocks.forEach(block => {
      const lines = countLines(block);
      if (lines > CONFIG.MAX_FUNC_LINES) {
        const funcName = block.match(/func\s+(?:\([^)]+\)\s+)?([A-Z]\w+)/)?.[1];
        errors.push(`⚠️  WARNING: Function '${funcName}' has ${lines} lines (max ${CONFIG.MAX_FUNC_LINES})`);
      }
    });
  }

  // Forbidden patterns
  CONFIG.FORBIDDEN_PATTERNS.forEach(pattern => {
    if (content.includes(pattern)) {
      errors.push(`⚠️  WARNING: Forbidden pattern '${pattern}' found`);
    }
  });

  // Directory-specific rules
  errors.push(...validateDirectory(filePath));

  // Logging validation
  errors.push(...validateLogging(content));

  // Error handling
  errors.push(...validateErrorHandling(content));

  // Duplicate detection
  errors.push(...checkForDuplicates(filePath, content));

  return errors;
}

// ============================================================================
// Hook Handlers
// ============================================================================

function handleUserPromptSubmit(data) {
  const userMessage = data.user_message || '';
  const isTestRelated = /test|tests|testing/i.test(userMessage);
  const isBuildRelated = /build|compile|rebuild/i.test(userMessage);

  if (isTestRelated || isBuildRelated) {
    console.log('═══════════════════════════════════════════════════════════');
    console.log('🚨 CRITICAL REMINDERS:');

    if (isTestRelated) {
      console.log('');
      console.log('  TESTS: MUST use Go-native test harness');
      console.log('         Runner: cd test && go run run_tests.go');
      console.log('         Direct: cd test && go test -v ./api or ./ui');
      console.log('         Unit:   go test ./internal/...');
    }

    if (isBuildRelated) {
      console.log('');
      console.log('  BUILD: MUST use ./scripts/build.ps1');
      console.log('         NEVER: go build directly');
      console.log('         Usage: ./scripts/build.ps1 [-Clean] [-Release]');
    }

    console.log('');
    console.log('═══════════════════════════════════════════════════════════');
  }

  process.exit(0);
}

function handlePreWrite(data) {
  const filePath = data.file_path || '';
  const content = data.content || '';

  if (!filePath.endsWith('.go')) {
    process.exit(0);
    return;
  }

  const errors = validateGoFile(filePath, content);

  if (errors.length > 0) {
    const blockers = errors.filter(e => e.includes('BLOCKED'));
    const warnings = errors.filter(e => e.includes('WARNING'));

    if (blockers.length > 0) {
      console.error('');
      console.error('═══════════════════════════════════════════════════════════');
      console.error('❌ WRITE OPERATION BLOCKED');
      console.error('═══════════════════════════════════════════════════════════');
      blockers.forEach(e => console.error(e));
      console.error('═══════════════════════════════════════════════════════════');
      process.exit(1);
    }

    if (warnings.length > 0) {
      console.warn('');
      warnings.forEach(w => console.warn(w));
      console.warn('');
    }
  }

  process.exit(0);
}

function handlePostWrite(data) {
  const filePath = data.file_path || '';

  if (filePath.endsWith('.go')) {
    // Update function index after successful write
    setTimeout(() => rebuildFunctionIndex(), 100);
  }

  process.exit(0);
}

// ============================================================================
// Main Entry Point
// ============================================================================

function main() {
  const args = process.argv.slice(2);
  const command = args[0];

  // Handle direct commands (e.g., rebuilding index)
  if (command === 'rebuild-index') {
    rebuildFunctionIndex();
    return;
  }

  // Handle hook events via stdin
  let inputData = '';

  process.stdin.on('data', chunk => inputData += chunk);
  process.stdin.on('end', () => {
    try {
      const data = JSON.parse(inputData);
      const hookType = data.hook_type || command;

      switch (hookType) {
        case 'user_prompt_submit':
          handleUserPromptSubmit(data);
          break;
        case 'pre_write':
        case 'pre_edit':
          handlePreWrite(data);
          break;
        case 'post_write':
        case 'post_edit':
          handlePostWrite(data);
          break;
        default:
          process.exit(0);
      }
    } catch (error) {
      // If parsing fails, allow operation
      process.exit(0);
    }
  });
}

main();
