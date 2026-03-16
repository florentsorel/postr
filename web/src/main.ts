import "./assets/main.css"
import { createApp } from "vue"
import { createPinia } from "pinia"
import { createRouter, createWebHistory } from "vue-router"
import ui from "@nuxt/ui/vue-plugin"
import App from "./App.vue"
import LibraryPage from "./pages/LibraryPage.vue"
import LoginPage from "./pages/LoginPage.vue"
import SettingsPage from "./pages/SettingsPage.vue"

const app = createApp(App)

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: LibraryPage },
    { path: "/login", component: LoginPage },
    { path: "/settings", component: SettingsPage },
  ],
})

app.use(createPinia())
app.use(router)
app.use(ui)
app.mount("#app")
