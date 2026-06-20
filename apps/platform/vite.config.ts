import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react-swc';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react(), tailwindcss()],
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
