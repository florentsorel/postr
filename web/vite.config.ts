import { defineConfig } from "vitest/config"
import vue from "@vitejs/plugin-vue"
import ui from "@nuxt/ui/vite"
import { fileURLToPath, URL } from "node:url"

export default defineConfig({
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
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
  },
})
