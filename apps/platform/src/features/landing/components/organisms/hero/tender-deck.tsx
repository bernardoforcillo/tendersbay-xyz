import { AnimatePresence, motion, useReducedMotion } from 'motion/react';
import { useEffect, useState } from 'react';
import { type Tender, TenderCard } from '~/features/landing/components/atoms';

const VISIBLE = 3;
const ADVANCE_MS = 2400;

/** Transform for a card sitting `d` slots behind the top of the deck. */
function depthStyle(d: number) {
  return { scale: 1 - d * 0.05, y: d * 12, opacity: 1 - d * 0.18 };
}

/**
 * An auto-playing, infinitely cycling deck of tender cards — a "Tinder" swipe loop:
 * the top card swipes off to the right and the next card rises forward, forever.
 * Cards stack via a single grid cell (no absolute/translate fight with motion's
 * own transforms). Renders a single static card under reduced-motion.
 */
export function TenderDeck({ tenders }: { tenders: Tender[] }) {
  const reduce = useReducedMotion();
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (reduce || tenders.length <= 1) return;
    const id = setInterval(() => setIndex((i) => i + 1), ADVANCE_MS);
    return () => clearInterval(id);
  }, [reduce, tenders.length]);

  if (reduce) {
    return (
      <div className="grid h-[280px] w-52 place-items-center">
        <TenderCard tender={tenders[0] as Tender} />
      </div>
    );
  }

  // A window of the next few tenders. The absolute position is the React key, so
  // the card that leaves the top unmounts (and swipes out) while a fresh card
  // mounts (and fades in) at the back.
  const deck = Array.from({ length: VISIBLE }, (_, d) => {
    const abs = index + d;
    return { abs, depth: d, tender: tenders[abs % tenders.length] as Tender };
  });

  return (
    <div className="grid h-[280px] w-52 place-items-center">
      <AnimatePresence>
        {deck.map(({ abs, depth, tender }) => (
          <motion.div
            key={abs}
            className="col-start-1 row-start-1"
            style={{ zIndex: VISIBLE - depth }}
            initial={{ ...depthStyle(VISIBLE - 1), opacity: 0 }}
            animate={depthStyle(depth)}
            exit={{ x: 320, rotate: 14, opacity: 0 }}
            transition={{ type: 'spring', stiffness: 300, damping: 30 }}
          >
            <TenderCard tender={tender} />
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
