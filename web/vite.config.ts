import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

const backend = 'http://127.0.0.1:19421';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: '../internal/app/webdist', emptyOutDir: true },
  server: {
    proxy: {
      '/api': {
        target: backend,
        changeOrigin: true,
        configure(proxy) {
          proxy.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Origin', backend);
          });
        },
      },
    },
  },
});
