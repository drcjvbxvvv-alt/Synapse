#!/usr/bin/env node
/**
 * P2-9: Bundle Size Budget Check
 *
 * Reads all *.js files under ui/dist/assets/ and enforces two budgets:
 *
 *   TOTAL_BUDGET_MB  — total JS across all chunks (default: 12 MB uncompressed)
 *   CHUNK_BUDGET_MB  — per-chunk limit for non-vendor chunks (default: 3 MB)
 *
 * Known large vendor chunks (monaco, antd, vendor) are excluded from the
 * per-chunk check but still count toward the total budget.
 *
 * Exit codes:
 *   0  — all budgets met
 *   1  — one or more budgets exceeded
 *
 * Usage:
 *   node scripts/check-bundle-size.mjs
 *   TOTAL_BUDGET_MB=10 node scripts/check-bundle-size.mjs
 */

import { readdirSync, statSync } from 'node:fs'
import { join, extname, basename } from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))
const DIST_ASSETS = join(__dirname, '..', 'dist', 'assets')

const TOTAL_BUDGET_MB  = parseFloat(process.env.TOTAL_BUDGET_MB  ?? '12')
const CHUNK_BUDGET_MB  = parseFloat(process.env.CHUNK_BUDGET_MB  ?? '3')

// Chunks whose names start with these prefixes are exempt from the per-chunk check.
const VENDOR_PREFIXES = ['monaco', 'antd', 'vendor', 'charts', 'i18n', 'query', 'router']

const MB = 1024 * 1024

// ── Collect JS files ────────────────────────────────────────────────────────

let files
try {
  files = readdirSync(DIST_ASSETS).filter(f => extname(f) === '.js')
} catch {
  console.error(`[bundle-size] ✗ dist/assets/ not found — run "npm run build" first`)
  process.exit(1)
}

if (files.length === 0) {
  console.error(`[bundle-size] ✗ No .js files found in dist/assets/`)
  process.exit(1)
}

// ── Measure ─────────────────────────────────────────────────────────────────

const chunks = files.map(f => {
  const fullPath = join(DIST_ASSETS, f)
  const size = statSync(fullPath).size
  const isVendor = VENDOR_PREFIXES.some(prefix => basename(f).startsWith(prefix))
  return { name: f, size, isVendor }
}).sort((a, b) => b.size - a.size)

const totalBytes = chunks.reduce((sum, c) => sum + c.size, 0)
const totalMB    = totalBytes / MB

// ── Print table ─────────────────────────────────────────────────────────────

console.log('\n[bundle-size] Chunk sizes (uncompressed):')
console.log('─'.repeat(72))
for (const c of chunks) {
  const mb    = (c.size / MB).toFixed(2)
  const flag  = c.isVendor ? '(vendor)' : ''
  const over  = !c.isVendor && c.size > CHUNK_BUDGET_MB * MB ? ' ← OVER BUDGET' : ''
  console.log(`  ${mb.padStart(6)} MB  ${c.name.padEnd(50)} ${flag}${over}`)
}
console.log('─'.repeat(72))
console.log(`  ${totalMB.toFixed(2).padStart(6)} MB  TOTAL`)
console.log()

// ── Check budgets ────────────────────────────────────────────────────────────

let failed = false

if (totalMB > TOTAL_BUDGET_MB) {
  console.error(
    `[bundle-size] ✗ Total JS (${totalMB.toFixed(2)} MB) exceeds budget of ${TOTAL_BUDGET_MB} MB`
  )
  failed = true
}

for (const c of chunks.filter(c => !c.isVendor)) {
  const mb = c.size / MB
  if (mb > CHUNK_BUDGET_MB) {
    console.error(
      `[bundle-size] ✗ Chunk "${c.name}" (${mb.toFixed(2)} MB) exceeds per-chunk budget of ${CHUNK_BUDGET_MB} MB`
    )
    failed = true
  }
}

if (!failed) {
  console.log(`[bundle-size] ✓ All budgets met (total: ${totalMB.toFixed(2)} MB / ${TOTAL_BUDGET_MB} MB)`)
}

process.exit(failed ? 1 : 0)
