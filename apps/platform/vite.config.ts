import { fileURLToPath, URL } from 'node:url';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    // `~` aliases the app's src/ directory: `~/App` -> `src/App`, bare `~` -> `src`.
    // Keep in sync with the `paths` entry in tsconfig.json.
    alias: {
      '~': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
  },
  build: {
    outDir: 'dist',
    // Keep emptyOutDir off so the committed dist/.gitkeep placeholder (which
    // //go:embed all:dist needs to compile on a clean checkout) survives builds.
    emptyOutDir: false,
  },
});
