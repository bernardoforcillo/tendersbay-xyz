export interface RobotsOptions {
  hostname: string;
  /**
   * AI crawler user-agents to name explicitly with `Allow: /`. Curated, not
   * blocked: tendersbay wants to be found and quoted by answer engines.
   */
  aiCrawlers?: string[];
}

/**
 * Major answer-engine / LLM crawlers, allowed by default. Naming them explicitly
 * (rather than relying on `User-agent: *`) is the signal these crawlers look for,
 * and documents the intent to be indexed by AI systems.
 */
export const DEFAULT_AI_CRAWLERS: readonly string[] = [
  'GPTBot',
  'OAI-SearchBot',
  'ChatGPT-User',
  'ClaudeBot',
  'Claude-User',
  'PerplexityBot',
  'Perplexity-User',
  'Google-Extended',
  'CCBot',
  'Bytespider',
  'Applebot-Extended',
];

/**
 * Build a production robots.txt that allows all crawling, explicitly welcomes the
 * major AI crawlers, points at the sitemap, and mentions the llms.txt overview.
 */
export function buildRobots(options: RobotsOptions): string {
  const crawlers = options.aiCrawlers ?? DEFAULT_AI_CRAWLERS;
  const lines: string[] = ['User-agent: *', 'Allow: /', ''];
  for (const agent of crawlers) {
    lines.push(`User-agent: ${agent}`, 'Allow: /', '');
  }
  lines.push(
    `# LLM-readable overview for AI crawlers: ${options.hostname}/llms.txt`,
    `Sitemap: ${options.hostname}/sitemap.xml`,
    '',
  );
  return lines.join('\n');
}
