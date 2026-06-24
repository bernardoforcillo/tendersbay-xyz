import { motion, type Variants } from 'motion/react';
import { cx } from '~/features/landing/cx';
import { type EuCountry, FLAGS } from './flags';

type CountryFlagProps = {
  code: EuCountry;
  name: string;
  available: boolean;
  statusLabel: string;
  variants?: Variants;
};

export function CountryFlag({ code, name, available, statusLabel, variants }: CountryFlagProps) {
  const Flag = FLAGS[code];
  const label = `${name} — ${statusLabel}`;
  return (
    <motion.li variants={variants} className="group relative flex">
      <span
        title={label}
        className={cx(
          'block w-full overflow-hidden rounded-md ring-1 transition duration-300',
          available
            ? 'opacity-100 shadow-soft ring-brand-300'
            : 'opacity-60 grayscale ring-cream-300 group-hover:-translate-y-0.5 group-hover:opacity-100 group-hover:shadow-soft group-hover:grayscale-0 motion-reduce:transform-none',
        )}
      >
        <Flag aria-hidden="true" className="block h-auto w-full" />
      </span>
      <span className="sr-only">{label}</span>
      {available ? (
        <span
          aria-hidden="true"
          className="absolute top-1 right-1 h-2 w-2 rounded-full bg-brand-500 ring-2 ring-white"
        />
      ) : null}
    </motion.li>
  );
}
