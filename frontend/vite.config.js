import react from '@vitejs/plugin-react';
import path from 'path';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

import { configDefaults } from 'vitest/config';

export default defineConfig(() => {
  return {
    base: '/',
    build: {
      outDir: 'build',
    },
    // Vite 8 switched from Rollup/esbuild to Rolldown/Oxc, which changed
    // how CommonJS default imports are resolved. This flag restores the
    // pre-Vite-8 behavior so that `import X from 'cjs-package'` resolves
    // to module.exports instead of module.exports.default. Used by
    // react-list (CJS) and any other CJS packages that don't export a
    // .default property.
    legacy: {
      inconsistentCjsInterop: true,
    },
    plugins: [
      react({
        jsxImportSource: '@emotion/react',
      }),

      VitePWA({
        registerType: 'autoUpdate',
        manifest: false,
        devOptions: {
          enabled: true,
        },
      }),
    ],
    server: {
      port: 3000,
      proxy: {
        '^(/auth|/fever|/greader|/version)': {
          target: 'http://goliath-dev:9999',
          changeOrigin: true,
          secure: false,
        },
      },
    },
    resolve: {
      tsconfigPaths: true,
      alias: {
        '@': path.resolve(__dirname, './src'),
      },
    },
    test: {
      environment: 'happy-dom',
      globals: true,
      exclude: [...configDefaults.exclude, '**/e2e/**'],
      setupFiles: ['./setupTests.ts'],
    },
  };
});
