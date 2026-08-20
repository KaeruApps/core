import { createRouter, createWebHistory } from "vue-router";

import HomePage from "./pages/HomePage.vue";
import EventLogPage from "./pages/EventLogPage.vue";
import SetupPage from "./pages/SetupPage.vue";
import OIDCSetupPage from "./pages/OIDCSetupPage.vue";
import LoginPage from "./pages/LoginPage.vue";
import { useSessionStore } from "./stores/session.js";

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
    {
      path: "/login",
      name: "login",
      component: LoginPage,
      meta: { authentication: true },
    },
    {
      path: "/setup",
      name: "setup",
      component: SetupPage,
      meta: { setup: true },
    },
    {
      path: "/setup/oidc",
      name: "setup-oidc",
      component: OIDCSetupPage,
      meta: { setup: true },
    },
  ],
});

const administratorRevokedMessage =
  "Your account no longer has Kaeru Core administrator access. You have been logged out.";

router.beforeEach(async (to) => {
  const session = useSessionStore();
  try {
    await session.refreshStatus();
    if (session.initialized) {
      window.sessionStorage.removeItem("kaeru.oidc-setup-draft");
    }
    if (!session.initialized && !to.meta.setup) {
      return { name: "setup" };
    }
    if (session.initialized && to.meta.setup) {
      return { name: "home" };
    }
    if (!session.initialized) {
      return true;
    }

    const authenticated = await session.refreshSession();
    if (authenticated && !session.isCoreAdministrator()) {
      await session.logoutQuietly();
      if (!to.meta.authentication) {
        return { name: "login", query: { error: administratorRevokedMessage } };
      }
      return true;
    }
    if (!authenticated && !to.meta.authentication) {
      return { name: "login", query: { redirect: to.fullPath } };
    }
    if (authenticated && to.meta.authentication) {
      return typeof to.query.redirect === "string" ? to.query.redirect : "/";
    }
  } catch {
    // Pages handle Core availability errors after navigation.
  }

  return true;
});
