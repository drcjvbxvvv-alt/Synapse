#!/usr/bin/env node
/**
 * i18n-lint.mjs
 *
 * Scans .tsx/.ts files under src/ for hardcoded Chinese characters.
 * Exits 1 if violations exceed the allowed threshold.
 *
 * Usage:
 *   node scripts/i18n-lint.mjs            # default threshold: 0
 *   MAX_VIOLATIONS=5 node scripts/i18n-lint.mjs
 *
 * Intentional exceptions (won't be flagged):
 *   - Lines containing // i18n-ignore
 *   - Comments (line comments and block comments)
 *   - console.log / console.error / console.warn strings
 *   - Import/export declarations
 *   - Lines inside locale JSON files themselves
 */

import { readdirSync, readFileSync, statSync } from 'fs';
import { join, relative, extname } from 'path';

const ROOT = new URL('..', import.meta.url).pathname;
const SRC  = join(ROOT, 'src');

// Chinese Unicode range: CJK Unified Ideographs + common punctuation
const ZH_PATTERN = /[\u4e00-\u9fff\u3400-\u4dbf\uff00-\uffef\u3000-\u303f]/;

// Lines to skip even if they contain Chinese
const SKIP_PATTERNS = [
  /\/\/.*i18n-ignore/,          // explicit ignore comment
  /^\s*\/\//,                   // full-line comment
  /^\s*\*/,                     // JSDoc / block comment line
  /^\s*import\s/,               // import statement
  /^\s*export\s/,               // export statement (re-exports)
  /console\.(log|warn|error)/,  // debug logging
  /^\s*\/\*\*/,                 // JSDoc start
];

// Patterns that typically appear in locale JSON keys or translation calls
const ALLOWED_PATTERNS = [
  /\bt\s*\(/,         // t('key') calls — the Chinese is already extracted
  /useTranslation/,   // hook declarations
];

/** Recursively collect .tsx and .ts files, skipping locales/ and node_modules/ */
function collectFiles(dir, files = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) {
      if (entry === 'node_modules' || entry === 'locales') continue;
      collectFiles(full, files);
    } else if (stat.isFile()) {
      const ext = extname(full);
      if (ext === '.tsx' || ext === '.ts') files.push(full);
    }
  }
  return files;
}

/** Scan one file; return array of { file, line, col, text } violations */
function scanFile(filePath) {
  const src = readFileSync(filePath, 'utf8');
  const lines = src.split('\n');
  const violations = [];
  let inBlockComment = false;

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];

    // Track block comments
    if (raw.includes('/*')) inBlockComment = true;
    if (raw.includes('*/')) { inBlockComment = false; continue; }
    if (inBlockComment) continue;

    // Skip patterns
    if (SKIP_PATTERNS.some(p => p.test(raw))) continue;

    if (!ZH_PATTERN.test(raw)) continue;

    // Skip lines that are purely t() calls or similar allowed patterns
    if (ALLOWED_PATTERNS.some(p => p.test(raw))) continue;

    // Find the column of the first Chinese character
    const col = raw.search(ZH_PATTERN);
    const snippet = raw.trim().slice(0, 120);

    violations.push({
      file: relative(ROOT, filePath),
      line: i + 1,
      col: col + 1,
      text: snippet,
    });
  }

  return violations;
}

// ── Main ────────────────────────────────────────────────────────────────────

const files = collectFiles(SRC);
const allViolations = [];

for (const f of files) {
  allViolations.push(...scanFile(f));
}

const MAX = parseInt(process.env.MAX_VIOLATIONS ?? '0', 10);

if (allViolations.length === 0) {
  console.log('✅  i18n-lint: no hardcoded Chinese found.');
  process.exit(0);
}

// Group by file for readable output
const byFile = {};
for (const v of allViolations) {
  (byFile[v.file] ??= []).push(v);
}

console.log(`\n⚠️  i18n-lint: found ${allViolations.length} hardcoded Chinese string(s)\n`);
for (const [file, viols] of Object.entries(byFile)) {
  console.log(`  ${file}`);
  for (const v of viols) {
    console.log(`    Line ${v.line}:${v.col}  ${v.text}`);
  }
}
console.log();

if (allViolations.length > MAX) {
  console.error(
    `❌  Violations (${allViolations.length}) exceed threshold (${MAX}). Fix or add // i18n-ignore.`,
  );
  process.exit(1);
}

console.log(`ℹ️  Violations (${allViolations.length}) within allowed threshold (${MAX}).`);
