import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    lib: {
      entry: './index.tsx',
      formats: ['es'],
      fileName: 'index',
    },
    rollupOptions: {
      external: [
        'solid-js',
        '@tanstack/solid-query',
        'graphql',
        'graphql-request',
      ],
    },
    outDir: 'dist',
    emptyOutDir: true,
  },
});
