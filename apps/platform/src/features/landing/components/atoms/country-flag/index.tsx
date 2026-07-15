import { cn } from '@tendersbay/components/core';
import { useState } from 'react';
import { Button, OverlayArrow, Tooltip, TooltipTrigger } from 'react-aria-components';
import { Icon } from '~/features/landing/components/atoms/icon';
import { type EuCountry, FLAGS } from './flags';

type CountryFlagProps = {
  code: EuCountry;
  name: string;
  portal: string;
  available: boolean;
  statusLabel: string;
  className?: string;
  /** Duplicate marquee copy: still hoverable, but kept out of the tab order. */
  decorative?: boolean;
};

export function CountryFlag({
  code,
  name,
  portal,
  available,
  statusLabel,
  className,
  decorative,
}: CountryFlagProps) {
  const Flag = FLAGS[code];
  const label = `${name} — ${statusLabel}`;
  // Controlled open: show the card instantly on hover/focus, bypassing
  // react-aria's global tooltip warmup (~1.5s) so the first hover isn't dead.
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);

  return (
    <TooltipTrigger
      isOpen={hovered || focused}
      onOpenChange={(open) => {
        if (!open) {
          setHovered(false);
          setFocused(false);
        }
      }}
    >
      <Button
        aria-label={label}
        excludeFromTabOrder={decorative}
        onHoverChange={setHovered}
        onFocusChange={setFocused}
        className={cn(
          'group/flag flex cursor-pointer flex-col gap-1.5 rounded-xl border bg-white p-1.5 outline-none transition duration-300',
          'hover:-translate-y-1 motion-reduce:transform-none',
          'data-[pressed]:translate-y-0 data-[pressed]:scale-[0.98]',
          'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-50',
          available
            ? 'border-brand-200 shadow-soft-md'
            : 'border-cream-200 shadow-soft hover:border-cream-300 hover:shadow-soft-md',
          className,
        )}
      >
        <span className="block overflow-hidden rounded-md ring-1 ring-ink-900/5">
          <Flag
            aria-hidden="true"
            className={cn(
              'block h-auto w-full transition duration-500',
              available
                ? ''
                : 'opacity-70 grayscale group-hover/flag:opacity-100 group-hover/flag:grayscale-0',
            )}
          />
        </span>
        <span className="flex items-center justify-between gap-1.5 px-0.5">
          <span
            title={name}
            className={cn(
              'min-w-0 flex-1 truncate text-left text-[11px] font-medium transition-colors duration-300',
              available ? 'text-brand-700' : 'text-ink-500 group-hover/flag:text-ink-700',
            )}
          >
            {name}
          </span>
          <span
            aria-hidden="true"
            className={cn(
              'h-1.5 w-1.5 shrink-0 rounded-full transition-colors duration-300',
              available ? 'bg-brand-500' : 'bg-cream-300 group-hover/flag:bg-brand-300',
            )}
          />
        </span>
      </Button>

      <Tooltip
        placement="top"
        offset={10}
        className={cn(
          'origin-bottom transition duration-200 ease-out',
          'data-[entering]:scale-95 data-[entering]:opacity-0',
          'data-[exiting]:scale-95 data-[exiting]:opacity-0',
        )}
      >
        <OverlayArrow className="group">
          <svg
            width={14}
            height={14}
            viewBox="0 0 14 14"
            aria-hidden="true"
            className="block fill-cream-50 group-data-[placement=bottom]:rotate-180 group-data-[placement=left]:-rotate-90 group-data-[placement=right]:rotate-90"
          >
            <path d="M0 0 L7 7 L14 0" />
          </svg>
        </OverlayArrow>
        <div className="w-60 rounded-2xl border border-cream-200 bg-cream-50 p-4 shadow-soft-lg">
          <div className="flex items-center gap-2.5">
            <span className="block w-9 shrink-0 overflow-hidden rounded ring-1 ring-ink-900/10">
              <Flag aria-hidden="true" className="block h-auto w-full" />
            </span>
            <h3 className="font-display text-base leading-tight text-ink-900">{name}</h3>
          </div>
          <p className="mt-3 flex items-center gap-1.5 font-mono text-sm font-medium text-brand-700">
            <Icon name="map" className="shrink-0 text-[15px] text-brand-500" />
            {portal}
          </p>
        </div>
      </Tooltip>
    </TooltipTrigger>
  );
}
