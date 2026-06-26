import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import {ElementPlusResolver} from 'unplugin-vue-components/resolvers'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  define: {
    __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: 'false',
    __VUE_OPTIONS_API__: 'true',
    __VUE_PROD_DEVTOOLS__: 'false',
  },
  plugins: [
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      imports: ['vue', 'vue-router', 'pinia'],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  css: {
    preprocessorOptions: {
      scss: {
        additionalData: `@use "@/assets/styles/variables.scss" as *;`,
      },
    },
  },
  resolve: {
    extensions: ['.mjs', '.js', '.ts', '.jsx', '.tsx', '.json'],
    alias: {
      '@': path.resolve(__dirname, 'src'),
      '@bindings': path.resolve(__dirname, 'bindings'),
      '../../wailsjs/go/main/App': path.resolve(__dirname, 'src/wails-compat/app.ts'),
      '../../wailsjs/go/models': path.resolve(__dirname, 'src/wails-compat/models.ts'),
      '../../wailsjs/runtime/runtime': path.resolve(__dirname, 'src/wails-compat/runtime.ts'),
    },
  },
  server: {
    watch: {
      ignored: ['**/bindings/**', '**/dist/**'],
    },
    fs: {
      allow: [path.resolve(__dirname)],
    },
  },
  optimizeDeps: {
    exclude: ['@bindings/*'],
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    include: ['src/**/*.{test,spec}.{js,ts}'],
  },
  build: {
    minify: 'terser',
  },
})
