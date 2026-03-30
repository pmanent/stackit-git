import {defineConfig} from 'vitest/config';
import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';

export default defineConfig({
  test: {
    include: ['web_src/**/*.test.js'],
    setupFiles: ['web_src/js/vitest.setup.js'],
    environment: 'happy-dom',
    testTimeout: 20000,
    open: false,
    allowOnly: true,
    passWithNoTests: true,
    globals: true,
    watch: false,
    mockReset: true,
    coverage: {
      provider: 'v8',
      include: ['web_src/**/*.{js,ts,vue}'],
      exclude: [
        'web_src/fomantic/**',
        'web_src/vendor/**',
        'web_src/**/*.test.{js,ts}',
        'web_src/js/vitest.setup.js',
      ],
    },
  },
  plugins: [
    stringPlugin(),
    vuePlugin(),
  ],
});
