import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { resolve } from 'node:path'

function packageNameFromId(id: string) {
  const parts = id.split('node_modules/')
  const packagePath = parts[parts.length - 1] || ''
  const normalized = packagePath.startsWith('.pnpm/')
    ? packagePath.split('/node_modules/').pop() || packagePath
    : packagePath
  const segments = normalized.split('/')
  if (normalized.startsWith('@')) {
    return `${segments[0]}/${segments[1]}`
  }
  return segments[0]
}

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: resolve(__dirname, '../web/dist'),
    emptyOutDir: true,
    chunkSizeWarningLimit: 850,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (
            id.includes('@ant-design+pro-components') ||
            id.includes('@ant-design/pro-components') ||
            id.includes('@ant-design+pro-') ||
            id.includes('@ant-design/pro-')
          ) {
            return 'ant-design-pro'
          }
          const packageName = packageNameFromId(id)
          if (['react', 'react-dom', 'scheduler', 'use-sync-external-store'].includes(packageName)) return 'react-core'
          if (['@tanstack/react-query'].includes(packageName)) return 'react-query'
          if (packageName === '@ant-design/pro-components' || packageName.startsWith('@ant-design/pro-')) return 'ant-design-pro'
          if (['@ant-design/icons', '@ant-design/icons-svg'].includes(packageName)) return 'ant-design-icons'
          if (
            packageName === '@ant-design/cssinjs' ||
            packageName === '@ant-design/cssinjs-utils' ||
            packageName === '@ant-design/colors' ||
            packageName === '@ant-design/fast-color' ||
            packageName === '@ant-design/react-slick' ||
            packageName === 'ahooks' ||
            packageName === 'antd-style' ||
            packageName === 'classnames' ||
            packageName === 'compute-scroll-into-view' ||
            packageName === 'copy-to-clipboard' ||
            packageName === 'json2mq' ||
            packageName === 'memoize-one' ||
            packageName === 'omit.js' ||
            packageName === 'rc-resize-observer' ||
            packageName === 'resize-observer-polyfill' ||
            packageName === 'scroll-into-view-if-needed' ||
            packageName === 'throttle-debounce' ||
            packageName.startsWith('@rc-component/') ||
            packageName.startsWith('rc-')
          ) {
            return 'ant-design-runtime'
          }
          if (packageName === 'antd') return 'ant-design'
          if (packageName === 'axios') return 'http-client'
          if (packageName === 'dayjs') return 'date-utils'
          return 'vendor'
        }
      }
    }
  },
  server: {
    port: 5173,
    host: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18080',
        changeOrigin: true
      }
    }
  }
})
