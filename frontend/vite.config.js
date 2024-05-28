import react from '@vitejs/plugin-react';
import path from 'path';
import {defineConfig} from 'vite';
import {VitePWA} from 'vite-plugin-pwa'
import viteTsconfigPaths from 'vite-tsconfig-paths';

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
      viteTsconfigPaths(),
      VitePWA({
        injectRegister: 'auto'
      })],
    server: {
      port: 3000,
      proxy: {
        "^(/version|/fever)": {
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
    }
  };
});