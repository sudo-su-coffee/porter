import { createRouter, createWebHashHistory } from "vue-router";
import { getToken } from "./api/client";

import DeploymentsList from "./views/DeploymentsList.vue";
import Analytics from "./views/Analytics.vue";
import ProjectDetail from "./views/ProjectDetail.vue";
import VmDetail from "./views/VmDetail.vue";
import Images from "./views/Images.vue";
import Servers from "./views/Servers.vue";
import Logs from "./views/Logs.vue";
import Traffic from "./views/Traffic.vue";
import Domains from "./views/Domains.vue";
import Teams from "./views/Teams.vue";
import Settings from "./views/Settings.vue";
import Login from "./views/Login.vue";
import Volumes from "./views/Volumes.vue";
import Feedback from "./views/Feedback.vue";

const routes = [
  { path: "/", name: "list", component: DeploymentsList },
  { path: "/analytics", name: "analytics", component: Analytics },
  { path: "/projects/:id", name: "project", component: ProjectDetail, props: true },
  { path: "/vms/:id", name: "vm", component: VmDetail, props: true },
  { path: "/traffic", name: "traffic", component: Traffic },
  { path: "/domains", name: "domains", component: Domains },
  { path: "/teams", name: "teams", component: Teams },
  { path: "/settings", name: "settings", component: Settings },
  { path: "/images", name: "images", component: Images },
  { path: "/servers", name: "servers", component: Servers },
  { path: "/logs", name: "logs", component: Logs },
  { path: "/login", name: "login", component: Login },
  { path: "/volumes", name: "volumes", component: Volumes },
  { path: "/feedback", name: "feedback", component: Feedback },
];

const router = createRouter({
  // Hash history keeps this a static-file-friendly single binary with
  // no server-side route handling required beyond serving index.html.
  history: createWebHashHistory(),
  routes,
});

router.beforeEach((to) => {
  if (to.name !== "login" && !getToken()) {
    return { name: "login" };
  }
});

export default router;
