#!/usr/bin/env node

/**
 * Intel Prune Hook (Stop event)
 *
 * Removes stale entries from index.json when files no longer exist.
 * Runs after each Claude response to keep intel fresh.
 *
 * Fast: Only does fs.existsSync checks, no file reading.
 * Silent: Never blocks or errors, always exits 0.
 */

const fs = require('fs');
const path = require('path');

function pruneIndex() {
  const intelDir = path.join(process.cwd(), '.planning', 'intel');
  const indexPath = path.join(intelDir, 'index.json');

  // Only run if intel directory exists (opt-in check)
  if (!fs.existsSync(intelDir)) {
    return { pruned: 0, total: 0 };
  }

  // Read existing index
  let index;
  try {
    const content = fs.readFileSync(indexPath, 'utf8');
    index = JSON.parse(content);
  } catch (e) {
    // No index or invalid JSON
    return { pruned: 0, total: 0 };
  }

  if (!index.files || typeof index.files !== 'object') {
    return { pruned: 0, total: 0 };
  }

  // Check each file and collect deleted ones
  const filePaths = Object.keys(index.files);
  const deleted = filePaths.filter(filePath => !fs.existsSync(filePath));

  if (deleted.length === 0) {
    return { pruned: 0, total: filePaths.length };
  }

  // Remove deleted entries
  for (const filePath of deleted) {
    delete index.files[filePath];
  }
  index.updated = Date.now();

  // Write updated index
  fs.writeFileSync(indexPath, JSON.stringify(index, null, 2));

  // Regenerate conventions and summary after pruning
  // Import detection logic from intel-index.js would be complex,
  // so we just update the index. Conventions/summary stay until
  // next PostToolUse or /gsd:analyze-codebase refresh.

  return { pruned: deleted.length, total: filePaths.length };
}

// Read JSON from stdin (standard hook pattern)
let input = '';
process.stdin.setEncoding('utf8');
process.stdin.on('data', chunk => input += chunk);
process.stdin.on('end', () => {
  try {
    // Stop hook receives session data, but we don't need it
    // Just prune stale entries
    pruneIndex();
    process.exit(0);
  } catch (error) {
    // Silent failure - never block Claude
    process.exit(0);
  }
});
