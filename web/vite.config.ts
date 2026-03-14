import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import ui from "@nuxt/ui/vite"

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
})
