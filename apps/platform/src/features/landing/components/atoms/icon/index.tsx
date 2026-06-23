import {
  ArrowRight,
  Check,
  ChevronDown,
  Clock,
  FileText,
  Globe,
  Layers,
  type LucideIcon,
  Map as MapIcon,
  Search,
  Sparkles,
  Trophy,
} from 'lucide-react';
import { cx } from '~/features/landing/cx';

export type IconName =
  | 'search'
  | 'document'
  | 'trophy'
  | 'arrow-right'
  | 'globe'
  | 'sparkle'
  | 'check'
  | 'map'
  | 'layers'
  | 'clock'
  | 'chevron-down';

const ICONS: Record<IconName, LucideIcon> = {
  search: Search,
  document: FileText,
  trophy: Trophy,
  'arrow-right': ArrowRight,
  globe: Globe,
  sparkle: Sparkles,
  check: Check,
  map: MapIcon,
  layers: Layers,
  clock: Clock,
  'chevron-down': ChevronDown,
};

type IconProps = { name: IconName; className?: string };

export function Icon({ name, className }: IconProps) {
  const LucideGlyph = ICONS[name];
  return (
    <LucideGlyph
      aria-hidden="true"
      strokeWidth={1.75}
      className={cx('h-[1em] w-[1em]', className)}
    />
  );
}
