import react from '@vitejs/plugin-react';
import path from 'path';
import { defineConfig } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

import { configDefaults } from 'vitest/config';

// Profiling headers are opt-in via GOLIATH_PROFILE=1 rather than always on.
// Cross-origin isolation is not a neutral setting: COEP changes how every
// cross-origin subresource is fetched, and the isolated context differs from
// what users actually run. Restricting it to measurement runs keeps ordinary
// development on the same footing as production.
const profilingEnabled = process.env.GOLIATH_PROFILE === '1';

const profilingHeaders = profilingEnabled
  ? {
      // Required to construct `new Profiler(...)` (JS Self-Profiling API).
      'Document-Policy': 'js-profiling',
      // Cross-origin isolation, which takes performance.now() from 100us to
      // 5us resolution — needed to resolve a ~20ms interaction. (It does not
      // help Profiler, whose sample interval stays pinned at a ~10ms floor
      // either way.) `credentialless` rather than `require-corp` so
      // cross-origin article preview images still load, without credentials,
      // and the measured workload stays realistic.
      'Cross-Origin-Opener-Policy': 'same-origin',
      'Cross-Origin-Embedder-Policy': 'credentialless',
    }
  : undefined;

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
        // src/index.tsx registers the worker itself via `virtual:pwa-register`,
        // which is what installs the reload-on-update handler. Without this the
        // plugin would also inject its own bare registration script.
        injectRegister: null,
        manifest: false,
        devOptions: {
          enabled: true,
        },
      }),
    ],
    server: {
      port: 3000,
      allowedHosts: true,
      headers: profilingHeaders,
      // The dev server runs in a container against a bind-mounted source tree,
      // and inotify events do not reliably cross that boundary. When one is
      // missed the module stays in the transform cache, so the dev server goes
      // on serving the old file — silently, and only for the modules that were
      // missed, which looks far more like a bug in the app than a stale build.
      // Polling costs a little idle CPU and buys back the guarantee that what
      // is on disk is what is being served.
      watch: {
        usePolling: true,
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
      headers: profilingHeaders,
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
