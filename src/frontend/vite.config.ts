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
    // 预打包 Element Plus 组件及其 CSS，避免首次访问懒加载视图时
    // Vite 发现新依赖触发整页 reload，导致第一次点击路由无响应
    include: (() => {
      const comps = [
        'alert', 'button', 'button-group', 'checkbox', 'checkbox-group',
        'dialog', 'dropdown', 'dropdown-item', 'dropdown-menu', 'empty',
        'form', 'form-item', 'icon', 'input', 'input-number', 'option',
        'popover', 'progress', 'radio-button', 'radio-group', 'select',
        'slider', 'switch', 'table', 'table-column', 'tab-pane', 'tabs',
        'tag', 'tooltip',
      ]
      return comps.flatMap(c => [
        `element-plus/es/components/${c}`,
        `element-plus/es/components/${c}/style/css`,
      ])
    })(),
  },
  test: {
    globals: true,
    environment: 'happy-dom',
    include: ['src/**/*.{test,spec}.{js,ts}'],
    server: {
      deps: {
        // element-plus 内部导入 theme-chalk/*.css，需经 Vite 管线处理
        // 否则 Node ESM loader 报 "Unknown file extension .css"
        inline: [/element-plus/],
      },
    },
  },
  build: {
    minify: 'terser',
  },
})
