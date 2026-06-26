import { Link } from 'react-aria-components';

export type FooterLink = { label: string; href: string };
type FooterColumnProps = { heading: string; links: FooterLink[] };

export function FooterColumn({ heading, links }: FooterColumnProps) {
  return (
    <nav aria-label={heading} className="flex flex-col gap-3.5">
      <h3 className="text-xs font-semibold uppercase tracking-[0.18em] text-ink-400">{heading}</h3>
      <ul className="flex flex-col gap-2.5">
        {links.map((link) => (
          <li key={link.label}>
            <Link
              href={link.href}
              className="text-sm text-ink-300 no-underline outline-none transition-colors data-[hovered]:text-white data-[focus-visible]:text-white data-[focus-visible]:underline"
            >
              {link.label}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
