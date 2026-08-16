import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// 本番では Go サーバが dist/ を配信するため、フロントエンドと API は常に
// 同一 Origin になる。開発時は Vite のサーバが /api を Go サーバへプロキシし、
// 同じ同一 Origin の状態を再現する。
export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.API_ORIGIN ?? 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
