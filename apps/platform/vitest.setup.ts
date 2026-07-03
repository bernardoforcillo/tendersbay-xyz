import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

afterEach(() => {
  cleanup();
});

// Node 24+ exposes a native `localStorage` global (unavailable without
// --localstorage-file) that shadows jsdom's Storage; vitest's populateGlobal
// won't override an already-defined global, so bare `localStorage` used by app
// code stays undefined. Bridge it to jsdom's real Storage (reachable via the
// jsdom window, with the `url` set in vitest.config.ts so Storage initializes).
// Note: the setup-scope `window` is not jsdom's window instance here, so we
// reach the working Storage through vitest's jsdom handle.
// biome-ignore lint/suspicious/noExplicitAny: reaching vitest's jsdom window for test env setup
const jsdomWindow = (global as any).jsdom?.window;
// Node 24+ exposes native localStorage/sessionStorage globals as undefined, shadowing jsdom's.
// Bridge both to jsdom's real Storage instances so Zustand persist works in tests.
if (typeof localStorage === 'undefined' && jsdomWindow?.localStorage) {
  vi.stubGlobal('localStorage', jsdomWindow.localStorage);
}
if (typeof sessionStorage === 'undefined' && jsdomWindow?.sessionStorage) {
  vi.stubGlobal('sessionStorage', jsdomWindow.sessionStorage);
}

// jsdom lacks matchMedia (used by reduced-motion checks) — provide a default mock.
if (!window.matchMedia) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));
}

// jsdom lacks IntersectionObserver (used by motion's whileInView / useInView).
class IntersectionObserverMock {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
  takeRecords(): [] {
    return [];
  }
}
vi.stubGlobal('IntersectionObserver', IntersectionObserverMock);

// jsdom lacks ResizeObserver (used by the marquee to measure its track width).
class ResizeObserverMock {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}
vi.stubGlobal('ResizeObserver', ResizeObserverMock);
