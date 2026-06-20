import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import type { PlopTypes } from '@turbo/gen';

/**
 * Atomic-design tiers. `pages` are app-specific routes, so they are only valid
 * for an app target — the guard in `actions` rejects `pages` for the shared
 * `@tendersbay/components` library.
 */
const TIERS = ['atoms', 'molecules', 'organisms', 'templates', 'pages'] as const;

const SHARED_BASE = 'packages/components/src';
const TARGETS = [
  { name: '@tendersbay/components (shared library)', value: SHARED_BASE },
  { name: 'platform app', value: 'apps/platform/src' },
];

const KEBAB_CASE = /^[a-z][a-z0-9]*(-[a-z0-9]+)*$/;
const kebab = (value: string) => KEBAB_CASE.test(value) || 'Use lowercase kebab-case';

interface Answers {
  base: string;
  feature: string;
  tier: string;
  name: string;
}

/**
 * Add an `export` line to a barrel file, creating it if needed. Idempotent:
 * the line is only added once, and the file is left as one export per line with
 * a trailing newline (Biome-clean).
 */
function ensureExport(filePath: string, line: string): string {
  const current = existsSync(filePath) ? readFileSync(filePath, 'utf8') : '';
  const lines = current
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean);
  if (lines.includes(line)) {
    return `unchanged ${filePath}`;
  }
  mkdirSync(path.dirname(filePath), { recursive: true });
  writeFileSync(filePath, `${[...lines, line].join('\n')}\n`);
  return `${existsSync(filePath) ? 'updated' : 'created'} ${filePath}`;
}

export default function generator(plop: PlopTypes.NodePlopAPI): void {
  const pascalCase = plop.getHelper('pascalCase') as (input: string) => string;

  plop.setGenerator('component', {
    description: 'Scaffold a feature-based, atomic-design React component',
    prompts: [
      {
        type: 'list',
        name: 'base',
        message: 'Target workspace:',
        choices: TARGETS,
      },
      {
        type: 'input',
        name: 'feature',
        message: 'Feature (kebab-case, e.g. tenders):',
        validate: kebab,
      },
      {
        type: 'list',
        name: 'tier',
        message: 'Atomic tier (pages = app target only):',
        choices: [...TIERS],
      },
      {
        type: 'input',
        name: 'name',
        message: 'Component (kebab-case, e.g. tender-card):',
        validate: kebab,
      },
    ],
    actions: (data) => {
      const { base, feature, tier, name } = data as Answers;
      if (base === SHARED_BASE && tier === 'pages') {
        throw new Error(
          '`pages` is not a shared component tier — scaffold pages into an app target instead.',
        );
      }
      const featureDir = path.join(base, feature);
      const tierDir = path.join(featureDir, 'components', tier);

      return [
        // The component itself: <base>/<feature>/components/<tier>/<name>/index.tsx
        {
          type: 'add',
          path: `${tierDir}/${name}/index.tsx`,
          templateFile: 'templates/component.tsx.hbs',
        },
        // Tier barrel re-exports the component.
        () =>
          ensureExport(
            path.join(tierDir, 'index.ts'),
            `export { ${pascalCase(name)} } from './${name}';`,
          ),
        // Feature barrel re-exports the tier (once).
        () =>
          ensureExport(path.join(featureDir, 'index.ts'), `export * from './components/${tier}';`),
      ];
    },
  });
}
