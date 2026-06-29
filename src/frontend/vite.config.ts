import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import {ElementPlusResolver} from 'unplugin-vue-components/resolvers'
import path from 'path'

// E2E 模式：Playwright webServer 通过 env 注入 E2E=true，使 Vite 将
// @bindings / @wailsio/runtime 别名指向 e2e/mocks，前端在不接入 Go 后端的情况下运行
const isE2E = process.env.E2E === 'true'

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
    // E2E 下用精确路径别名把 @bindings/yx-daq/internal/{app,types} 和
    // @wailsio/runtime 重定向到 e2e/mocks（mock 目录结构不必镜像真实 bindings）
    alias: isE2E
      ? [
          { find: '@', replacement: path.resolve(__dirname, 'src') },
          { find: '@bindings/yx-daq/internal/app', replacement: path.resolve(__dirname, 'e2e/mocks/bindings/app') },
          { find: '@bindings/yx-daq/internal/types', replacement: path.resolve(__dirname, 'e2e/mocks/bindings/types') },
          { find: '@wailsio/runtime', replacement: path.resolve(__dirname, 'e2e/mocks/wails-runtime.ts') },
        ]
      : {
          '@': path.resolve(__dirname, 'src'),
          '@bindings': path.resolve(__dirname, 'bindings'),
        },
  },
  server: {
    port: isE2E ? 5174 : 5173,
    strictPort: true,
    watch: {
      ignored: ['**/bindings/**', '**/dist/**', ...(isE2E ? ['**/e2e/**'] : [])],
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
