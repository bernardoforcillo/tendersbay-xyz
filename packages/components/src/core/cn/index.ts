import { type ClassValue, clsx } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

// The theme's soft elevation scale (shadow-soft, shadow-soft-md, shadow-soft-lg)
// isn't a stock Tailwind shadow, so tailwind-merge would misread it as a shadow
// *color* and merge against real shadows incorrectly.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      shadow: [{ shadow: ['soft', 'soft-md', 'soft-lg'] }],
    },
  },
});

/** shadcn-style class combiner: clsx composition + tailwind-merge conflict resolution. */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
