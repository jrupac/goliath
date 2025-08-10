import react from '@vitejs/plugin-react';
import path from 'path';
import {defineConfig} from 'vite';
import {VitePWA} from 'vite-plugin-pwa'
import viteTsconfigPaths from 'vite-tsconfig-paths';
import {configDefaults} from 'vitest/config';

export default defineConfig(() => {
  return {
    base: "/",
    build: {
      outDir: 'build',
    },
    plugins: [
      react({
        jsxImportSource: '@emotion/react',
        babel: {
          plugins: ['@emotion/babel-plugin'],
        },
      }),
      viteTsconfigPaths({
        // Ignore errors from tsconfig files from dependencies
        ignoreConfigErrors: true,
      }),
      VitePWA({
        registerType: 'autoUpdate',
        manifest: false,
        devOptions: {
          enabled: true
        }
      })],
    server: {
      port: 3000,
      proxy: {
        "^(/auth|/fever|/greader|/version)": {
          target: "http://goliath-dev:9999",
          changeOrigin: true,
          secure: false,
        }
      }
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      }
    },
    test: {
      environment: 'happy-dom',
      globals: true,
      exclude: [...configDefaults.exclude, '**/e2e/**'],
      setupFiles: ['./setupTests.ts']
    }
  };
});