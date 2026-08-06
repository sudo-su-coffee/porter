import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import { setUnauthorizedHandler } from "./api/client";
import "./style.css";

setUnauthorizedHandler(() => {
  router.push({ name: "login" });
});

createApp(App).use(router).mount("#app");
