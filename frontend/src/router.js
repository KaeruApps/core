import { createRouter, createWebHistory } from "vue-router";

import HomePage from "./pages/HomePage.vue";
import EventLogPage from "./pages/EventLogPage.vue";

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: "/",
      name: "home",
      component: HomePage,
    },
    {
      path: "/events",
      name: "events",
      component: EventLogPage,
    },
  ],
});
