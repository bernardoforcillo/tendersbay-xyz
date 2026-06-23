// Brings the @testing-library/jest-dom matcher augmentation (e.g. toBeInTheDocument)
// into the tsc program. The runtime registration lives in `vitest.setup.ts`, which sits
// outside `src` (the only tsconfig include), so this ambient file makes the matcher types
// visible to `tsc --noEmit`.
import '@testing-library/jest-dom/vitest';
