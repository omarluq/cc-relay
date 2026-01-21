#!/usr/bin/env node
// Codebase Intelligence - SessionStart Context Injection Hook
// Reads pre-generated summary.md and injects into Claude's context

const fs = require('fs');
const path = require('path');

// Read JSON from stdin (standard hook pattern)
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  try {
    const data = JSON.parse(input);

    // Only inject on startup or resume
    if (!['startup', 'resume'].includes(data.source)) {
      process.exit(0);
    }

    // Read pre-generated summary (created by gsd-intel-index.js)
    const summaryPath = path.join(process.cwd(), '.planning', 'intel', 'summary.md');

    if (!fs.existsSync(summaryPath)) {
      process.exit(0);  // No intel, skip silently
    }

    const summary = fs.readFileSync(summaryPath, 'utf8').trim();

    if (summary) {
      process.stdout.write(`<codebase-intelligence>\n${summary}\n</codebase-intelligence>`);
    }

    process.exit(0);
  } catch (error) {
    // Silent failure - never block Claude
    process.exit(0);
  }
});
