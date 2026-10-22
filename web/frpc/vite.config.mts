import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import svgLoader from 'vite-svg-loader'

// https://vitejs.dev/config/
export default defineConfig({
  base: '',
  plugins: [
    vue(),
    svgLoader(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@shared': fileURLToPath(new URL('../shared', import.meta.url)),
    },
    dedupe: ['vue', '@arco-design/web-vue'],
  },
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@shared/css/_index.scss" as *;`,
      },
    },
  },
  build: {
    assetsDir: '',
    chunkSizeWarningLimit: 1000,
    minify: 'terser',
    rollupOptions: {
      output: {
        manualChunks: {
          arco: ['@arco-design/web-vue'],
          codemirror: ['@codemirror/state', '@codemirror/view'],
        },
      },
    },
    terserOptions: {
      compress: {
        drop_console: true,
        drop_debugger: true,
      },
    },
  },
  server: {
    allowedHosts: process.env.ALLOWED_HOSTS
      ? process.env.ALLOWED_HOSTS.split(',')
      : [],
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://127.0.0.1:7400',
        changeOrigin: true,
      },
    },
  },
})
