import { motion, type Variants } from 'motion/react';
import { Button, Dialog, DialogTrigger, Popover } from 'react-aria-components';
import { cx } from '~/features/landing/cx';
import { type EuCountry, FLAGS } from './flags';

type CountryFlagProps = {
  code: EuCountry;
  name: string;
  portal: string;
  available: boolean;
  statusLabel: string;
  variants?: Variants;
};

export function CountryFlag({
  code,
  name,
  portal,
  available,
  statusLabel,
  variants,
}: CountryFlagProps) {
  const Flag = FLAGS[code];
  const label = `${name} — ${statusLabel}`;

  return (
    <motion.li variants={variants} className="group relative flex">
      <DialogTrigger>
        <Button
          aria-label={label}
          className={cx(
            'block w-full cursor-pointer overflow-hidden rounded-md ring-1 outline-none transition duration-300',
            'data-[focus-visible]:ring-2 data-[focus-visible]:ring-brand-600 data-[focus-visible]:ring-offset-2 data-[focus-visible]:ring-offset-cream-50',
            available
              ? 'opacity-100 shadow-soft ring-brand-300'
              : 'opacity-60 grayscale ring-cream-300 group-hover:-translate-y-0.5 group-hover:opacity-100 group-hover:shadow-soft group-hover:grayscale-0 data-[pressed]:scale-95 motion-reduce:transform-none',
          )}
        >
          <Flag aria-hidden="true" className="block h-auto w-full" />
        </Button>

        <Popover
          placement="top"
          offset={10}
          className={cx(
            'origin-bottom transition duration-200 ease-out',
            'data-[entering]:scale-95 data-[entering]:opacity-0',
            'data-[exiting]:scale-95 data-[exiting]:opacity-0',
          )}
        >
          <Dialog
            aria-label={name}
            className="w-60 rounded-2xl border border-cream-200 bg-cream-50 p-4 shadow-soft-lg outline-none"
          >
            <div className="flex items-center gap-2.5">
              <span className="block w-8 shrink-0 overflow-hidden rounded ring-1 ring-cream-300">
                <Flag aria-hidden="true" className="block h-auto w-full" />
              </span>
              <h3 className="font-display text-base text-ink-900">{name}</h3>
            </div>
            <p className="mt-3 font-mono text-sm font-medium text-brand-700">{portal}</p>
            <span
              className={cx(
                'mt-3 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 font-mono text-[10px] font-semibold uppercase tracking-[0.14em]',
                available
                  ? 'bg-brand-50 text-brand-700 ring-1 ring-brand-200'
                  : 'bg-cream-200 text-ink-600',
              )}
            >
              <span
                className={cx(
                  'h-1.5 w-1.5 rounded-full',
                  available ? 'bg-brand-500' : 'bg-ink-400',
                )}
              />
              {statusLabel}
            </span>
          </Dialog>
        </Popover>
      </DialogTrigger>

      {available ? (
        <span
          aria-hidden="true"
          className="pointer-events-none absolute top-1 right-1 z-10 h-2 w-2 rounded-full bg-brand-500 ring-2 ring-white"
        />
      ) : null}
    </motion.li>
  );
}
