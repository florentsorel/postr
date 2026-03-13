import "./assets/main.css"
import { createApp } from "vue"
import { createRouter, createWebHistory } from "vue-router"
import ui from "@nuxt/ui/vue-plugin"
import App from "./App.vue"
import LoginPage from "./pages/LoginPage.vue"
import SettingsPage from "./pages/SettingsPage.vue"

const app = createApp(App)

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", component: LoginPage },
    { path: "/settings", component: SettingsPage },
  ],
})

app.use(router)
app.use(ui)
app.mount("#app")
