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
      allowedHosts: true,
      headers: {
        // Enables the JS Self-Profiling API (`new Profiler(...)`) so the
        // dev-only profiling harness in tools/ can capture sampled stacks.
        // Dev server only — never sent by the production backend.
        'Document-Policy': 'js-profiling',
        // Cross-origin isolation lifts Chrome's timer coarsening, taking
        // performance.now() from 100us to 5us resolution — the keypress
        // benchmark needs that to resolve a ~20ms interaction. (It does not
        // help Profiler, whose sample interval stays pinned at a ~10ms floor
        // either way.) `credentialless` rather than `require-corp` so
        // cross-origin article preview images still load, without credentials,
        // and the measured workload stays realistic.
        'Cross-Origin-Opener-Policy': 'same-origin',
        'Cross-Origin-Embedder-Policy': 'credentialless',
      },
      proxy: {
        '^(/auth|/fever|/greader|/version)': {
          target: 'http://goliath-dev:9999',
          changeOrigin: true,
          secure: false,
        },
      },
    },
    // `vite preview` serves the production build. Mirrors the dev server's
    // proxy and profiling headers so the keypress benchmark can measure a
    // realistic production bundle rather than the instrumented dev one.
    preview: {
      port: 4173,
      allowedHosts: true,
      headers: {
        'Document-Policy': 'js-profiling',
        'Cross-Origin-Opener-Policy': 'same-origin',
        'Cross-Origin-Embedder-Policy': 'credentialless',
      },
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
