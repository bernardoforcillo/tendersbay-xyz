import { existsSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

export interface DiscoverOptions {
  include?: string[];
  exclude?: string[];
}

/** List files under `dir`, returning POSIX-separated paths relative to it. */
function listFiles(root: string, sub = ''): string[] {
  const current = sub ? join(root, sub) : root;
  const files: string[] = [];
  for (const entry of readdirSync(current, { withFileTypes: true })) {
    const rel = sub ? `${sub}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      files.push(...listFiles(root, rel));
    } else {
      files.push(rel);
    }
  }
  return files;
}

/** Map a route file (relative to the `$locale` dir) to a locale-relative path, or null to skip. */
function fileToPath(relativeFile: string): string | null {
  const withoutExt = relativeFile.replace(/\.(tsx?|jsx?)$/, '');
  const segments = withoutExt.split('/').flatMap((segment) => segment.split('.'));
  if (segments.at(-1) === 'route') {
    return null;
  }
  if (segments.some((segment) => segment.startsWith('$'))) {
    return null;
  }
  const isIndex = segments.at(-1) === 'index';
  const pathSegments = isIndex ? segments.slice(0, -1) : segments;
  const base = `/${pathSegments.join('/')}`.replace(/\/+/g, '/');
  if (isIndex) {
    return base === '/' ? '/' : `${base}/`;
  }
  return base;
}

/** Match a path against a list of globs where `*` matches any run of characters. */
function matchesAny(globs: string[], path: string): boolean {
  return globs.some((glob) => {
    const pattern = glob.replace(/[.+?^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*');
    return new RegExp(`^${pattern}$`).test(path);
  });
}

/** Discover public, static, locale-relative routes under `<routesDir>/$locale`. */
export function discoverRoutes(routesDir: string, options: DiscoverOptions = {}): string[] {
  const localeRoot = join(routesDir, '$locale');
  const paths = new Set<string>();
  if (existsSync(localeRoot)) {
    for (const file of listFiles(localeRoot)) {
      const path = fileToPath(file);
      if (path) {
        paths.add(path);
      }
    }
  }
  for (const extra of options.include ?? []) {
    paths.add(extra);
  }
  const exclude = options.exclude ?? [];
  return [...paths].filter((path) => !matchesAny(exclude, path)).sort();
}
