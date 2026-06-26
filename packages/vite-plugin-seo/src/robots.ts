export interface RobotsOptions {
  hostname: string;
}

/** Build a production robots.txt that allows all crawling and points at the sitemap. */
export function buildRobots(options: RobotsOptions): string {
  return ['User-agent: *', 'Allow: /', '', `Sitemap: ${options.hostname}/sitemap.xml`, ''].join(
    '\n',
  );
}
