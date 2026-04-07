import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
import Components from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    Components({
      dts: resolve(__dirname, 'src/components.d.ts'),
      resolvers: [
        AntDesignVueResolver({
          importStyle: false
        })
      ]
    })
  ],
  build: {
    outDir: resolve(__dirname, '../web/dist'),
    emptyOutDir: true,
    chunkSizeWarningLimit: 950,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('@ant-design/icons-vue') || id.includes('ant-design-vue')) {
            return 'vendor-ant'
          }
          if (id.includes('/pinia/')) {
            return 'vendor-pinia'
          }
          if (id.includes('/axios/')) {
            return 'vendor-axios'
          }
          return 'vendor-misc'
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
