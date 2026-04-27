import { defineConfig } from "vitest/config"
import vue from "@vitejs/plugin-vue"
import ui from "@nuxt/ui/vite"
import { fileURLToPath, URL } from "node:url"
import { readFileSync } from "node:fs"

const pkg = JSON.parse(readFileSync(new URL("./package.json", import.meta.url), "utf-8")) as {
  version: string
}

export default defineConfig({
  define: {
    "import.meta.env.VITE_APP_VERSION": JSON.stringify(pkg.version),
  },
  plugins: [
    vue(),
    ui({
      ui: {
        colors: {
          primary: "plex",
          neutral: "zinc",
          movie: "plex",
          show: "blue",
          season: "violet",
          collection: "emerald",
        },
      },
      theme: {
        colors: [
          "primary",
          "secondary",
          "success",
          "info",
          "warning",
          "error",
          "neutral",
          "movie",
          "show",
          "season",
          "collection",
        ],
      },
    }),
  ],
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        configure: (proxy) => {
          proxy.on("proxyRes", (proxyRes) => {
            if (proxyRes.headers["content-type"]?.startsWith("text/event-stream")) {
              proxyRes.headers["x-accel-buffering"] = "no"
            }
          })
        },
      },
    },
  },
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    // @vue/test-utils >= 2.4.7 registers `attachTo.removeChild(el)` in
    // onUnmount, but @testing-library/vue@8.1.0's `render` strips that
    // wrapper via `unwrapNode` right after mount — so VTU's onUnmount
    // throws "The node to be removed is not a child of this node" during
    // the auto-cleanup, marking every test as failed. Skip auto-cleanup
    // and clear `document.body` ourselves in setup.ts.
    env: {
      VTL_SKIP_AUTO_CLEANUP: "true",
    },
  },
})
