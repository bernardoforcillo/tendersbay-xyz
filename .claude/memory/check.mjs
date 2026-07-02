#!/usr/bin/env node
// Completeness checker for the .claude/memory wiki.
// Exit 0 = consistent; exit 1 = problems (printed).
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const dir = dirname(fileURLToPath(import.meta.url));
const SPECIAL = new Set(['index.md', 'log.md']);
const problems = [];

const files = readdirSync(dir).filter((f) => f.endsWith('.md'));
const pages = files.filter((f) => !SPECIAL.has(f));
const slugs = new Set(pages.map((f) => f.replace(/\.md$/, '')));

// (a) + (c) + (d): per-page frontmatter + name/slug match
for (const file of pages) {
  const slug = file.replace(/\.md$/, '');
  const text = readFileSync(join(dir, file), 'utf8');
  const fm = text.match(/^---\n([\s\S]*?)\n---/);
  if (!fm) {
    problems.push(`${file}: missing frontmatter block`);
    continue;
  }
  const block = fm[1];
  if (!/^name:\s*\S/m.test(block)) problems.push(`${file}: missing 'name'`);
  if (!/^description:\s*\S/m.test(block)) problems.push(`${file}: missing 'description'`);
  if (!/^\s*type:\s*\S/m.test(block)) problems.push(`${file}: missing 'metadata.type'`);
  const nameMatch = block.match(/^name:\s*(\S+)/m);
  if (nameMatch && nameMatch[1] !== slug)
    problems.push(`${file}: name '${nameMatch[1]}' != slug '${slug}'`);
}

// (b): every [[link]] resolves
for (const file of files) {
  const text = readFileSync(join(dir, file), 'utf8');
  for (const m of text.matchAll(/\[\[([a-z0-9-]+)\]\]/g)) {
    if (!slugs.has(m[1])) problems.push(`${file}: dangling link [[${m[1]}]]`);
  }
}

// (a): index lists every page exactly once
const index = readFileSync(join(dir, 'index.md'), 'utf8');
for (const slug of slugs) {
  const count = (index.match(new RegExp(`\\]\\(${slug}\\.md\\)`, 'g')) || []).length;
  if (count === 0) problems.push(`index.md: missing entry for ${slug}.md`);
  if (count > 1) problems.push(`index.md: duplicate entry for ${slug}.md (${count}x)`);
}
for (const m of index.matchAll(/\]\(([a-z0-9-]+)\.md\)/g)) {
  if (!slugs.has(m[1])) problems.push(`index.md: entry for missing page ${m[1]}.md`);
}

if (problems.length) {
  console.error(`memory wiki: ${problems.length} problem(s):`);
  for (const p of problems) console.error(`  - ${p}`);
  process.exit(1);
}
console.log(`memory wiki OK: ${pages.length} pages, index + links consistent`);
