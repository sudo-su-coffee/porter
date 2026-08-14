import { createApp } from "vue";
import App from "./App.vue";
import router from "./router";
import { setUnauthorizedHandler } from "./api/client";
import { initSentry } from "./observability";
import "./style.css";

setUnauthorizedHandler(() => {
  router.push({ name: "login" });
});

const app = createApp(App);
initSentry(app, router);
app.use(router).mount("#app");
