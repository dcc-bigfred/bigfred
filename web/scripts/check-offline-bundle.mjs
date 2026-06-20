#!/usr/bin/env node
/**
 * Fails the build if dist/ would load UI assets from the Internet at runtime.
 * See docs/content/bigfred/architecture/09b-offline-assets.md
 */
import { readdir, readFile, stat } from "node:fs/promises";
import path from "node:path";

const distDir = process.argv[2] ?? "dist";

const forbiddenHosts = [
  "fonts.googleapis.com",
  "fonts.gstatic.com",
  "unpkg.com",
  "cdn.jsdelivr.net",
  "cdnjs.cloudflare.com",
  "ajax.googleapis.com",
  "esm.sh",
  "skypack.dev",
];

const scanExtensions = new Set([".html", ".js", ".css", ".json"]);

const htmlExternalTag =
  /<(?:link|script|img|iframe)[^>]+(?:href|src)=["']https?:\/\//gi;
const cssExternalImport = /@import\s+["']https?:\/\//;
const cssExternalUrl = /url\(\s*["']?https?:\/\//;
const dynamicImport = /import\s*\(\s*["']https?:\/\//;
const newWorker = /new\s+Worker\s*\(\s*["']https?:\/\//;

function hostViolations(text) {
  const hits = [];
  for (const host of forbiddenHosts) {
    if (text.includes(host)) {
      hits.push(host);
    }
  }
  return hits;
}

function fileViolations(file, text) {
  const rel = path.relative(distDir, file);
  const ext = path.extname(file);
  const reasons = [];

  for (const host of hostViolations(text)) {
    reasons.push(`forbidden host ${host}`);
  }

  if (ext === ".html") {
    if (htmlExternalTag.test(text)) {
      reasons.push("external <link>/<script>/<img> in HTML");
    }
    htmlExternalTag.lastIndex = 0;
  }

  if (ext === ".css") {
    if (cssExternalImport.test(text)) {
      reasons.push("external @import in CSS");
    }
    if (cssExternalUrl.test(text)) {
      reasons.push("external url() in CSS");
    }
  }

  if (ext === ".js") {
    if (dynamicImport.test(text)) {
      reasons.push("dynamic import() of external URL");
    }
    if (newWorker.test(text)) {
      reasons.push("Worker loaded from external URL");
    }
  }

  return reasons.length > 0 ? { file: rel, reasons } : null;
}

async function walk(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  const files = [];
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walk(full)));
      continue;
    }
    if (scanExtensions.has(path.extname(entry.name))) {
      files.push(full);
    }
  }
  return files;
}

async function main() {
  try {
    const info = await stat(distDir);
    if (!info.isDirectory()) {
      throw new Error(`${distDir} is not a directory`);
    }
  } catch {
    console.error(`check-offline-bundle: missing ${distDir} — run vite build first`);
    process.exit(1);
  }

  const violations = [];
  for (const file of await walk(distDir)) {
    const text = await readFile(file, "utf8");
    const hit = fileViolations(file, text);
    if (hit) {
      violations.push(hit);
    }
  }

  if (violations.length > 0) {
    console.error("offline bundle check FAILED — runtime CDN / external asset refs:\n");
    for (const v of violations) {
      console.error(`  ${v.file}: ${v.reasons.join(", ")}`);
    }
    console.error(
      "\nBundle must not load fonts, scripts or styles from the Internet at runtime.",
    );
    process.exit(1);
  }

  console.log(`offline bundle OK (${distDir})`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
