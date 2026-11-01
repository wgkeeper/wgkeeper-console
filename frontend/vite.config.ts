import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../backend/public'),
    emptyOutDir: true,
    sourcemap: false,
    rolldownOptions: {
      output: {
        manualChunks: (id: string) => {
          if (/node_modules\/(react|react-dom|react-router|scheduler)\//u.test(id)) {
            return 'vendor-react';
          }
          // Radix, cmdk and lucide stay out of a shared vendor chunk on
          // purpose: most of them are only used by lazy routes, and grouping
          // them would preload all of it with the first page.
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8000',
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
});
