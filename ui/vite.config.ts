import path from 'node:path'
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'
import { visualizer } from 'rollup-plugin-visualizer'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const rootEnv = loadEnv(mode, path.resolve(__dirname, '..'), '')
  const backendPort = process.env.SERVER_PORT || rootEnv.SERVER_PORT || '8080'
  const backendHttp = `http://127.0.0.1:${backendPort}`
  const backendWs = `ws://127.0.0.1:${backendPort}`

  // P2-9: Enable bundle visualizer with VISUALIZE=true npm run build
  const enableVisualizer = process.env.VISUALIZE === 'true'

  return {
    plugins: [
      react(),
      // Generates dist/stats.html — run with: VISUALIZE=true npm run build
      ...(enableVisualizer ? [visualizer({
        filename: 'dist/stats.html',
        gzipSize: true,
        brotliSize: true,
        open: false,
      })] : []),
    ],
    resolve: {
      alias: {
        '@': path.resolve(__dirname, 'src'),
      },
    },
    server: {
      host: '0.0.0.0',
      port: 5173,
      proxy: {
        '/api': {
          target: backendHttp,
          changeOrigin: true,
          secure: false,
        },
        '/ws': {
          target: backendWs,
          ws: true,
          changeOrigin: true,
        },
      },
    },
    optimizeDeps: {
      include: ['monaco-editor'],
    },
    build: {
      outDir: 'dist',
      emptyOutDir: true,
      sourcemap: false,
      commonjsOptions: {
        include: [/monaco-editor/, /node_modules/],
      },
      rollupOptions: {
        output: {
          // P2-9: Fine-grained manual chunks to reduce initial bundle size.
          // Heavy vendor libraries are split so they can be loaded on demand.
          manualChunks(id) {
            // Monaco editor — largest single dep (~4MB uncompressed)
            if (id.includes('monaco-editor') || id.includes('@monaco-editor')) {
              return 'monaco'
            }
            // Charting libraries
            if (id.includes('recharts') || id.includes('d3-') || id.includes('victory')) {
              return 'charts'
            }
            // Ant Design
            if (id.includes('antd') || id.includes('@ant-design')) {
              return 'antd'
            }
            // React core
            if (id.includes('react-dom') || id.includes('/react/')) {
              return 'vendor'
            }
            // i18n
            if (id.includes('i18next') || id.includes('react-i18next')) {
              return 'i18n'
            }
            // Tanstack Query
            if (id.includes('@tanstack')) {
              return 'query'
            }
            // Router
            if (id.includes('react-router')) {
              return 'router'
            }
          },
        },
      },
    },
  }
})
