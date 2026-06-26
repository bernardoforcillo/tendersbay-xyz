import { useEffect, useState } from 'react';

const ROTATE_MS = 2800;

/**
 * Cycles through `examples` on a fixed interval, returning the current one.
 * No rotation when `enabled` is false or there is a single example, so a
 * reduced-motion user sees a stable first example.
 */
export function useRotatingPlaceholder(examples: string[], enabled: boolean): { example: string } {
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (!enabled || examples.length <= 1) return;
    const id = setInterval(() => {
      setIndex((current) => (current + 1) % examples.length);
    }, ROTATE_MS);
    return () => clearInterval(id);
  }, [enabled, examples.length]);

  const example = examples[index % examples.length] ?? examples[0] ?? '';
  return { example };
}
