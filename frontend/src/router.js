import { createRouter, createWebHistory } from "vue-router";

import HomePage from "./pages/HomePage.vue";
import EventLogPage from "./pages/EventLogPage.vue";
import SetupPage from "./pages/SetupPage.vue";
import OIDCSetupPage from "./pages/OIDCSetupPage.vue";
import LoginPage from "./pages/LoginPage.vue";

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

router.beforeEach(async (to) => {
  try {
    const response = await fetch("/api/v1/setup/status", {
      headers: { Accept: "application/json" },
    });
    if (!response.ok) {
      return true;
    }

    const status = await response.json();
    if (status.initialized) {
      window.sessionStorage.removeItem("kaeru.oidc-setup-draft");
    }
    if (!status.initialized && !to.meta.setup) {
      return { name: "setup" };
    }
    if (status.initialized && to.meta.setup) {
      return { name: "home" };
    }
    if (status.initialized) {
      const sessionResponse = await fetch("/api/v1/session", {
        headers: { Accept: "application/json" },
      });
      const authenticated = sessionResponse.ok;
      const session = authenticated ? await sessionResponse.json() : null;
      const coreAdministrator = session?.user?.service_roles?.core === "admin";
      if (authenticated && !coreAdministrator) {
        await fetch("/api/v1/session/logout", {
          method: "POST",
          headers: { Accept: "application/json" },
        });
        if (!to.meta.authentication) {
          return {
            name: "login",
            query: {
              error: "Your account no longer has Kaeru Core administrator access. You have been logged out.",
            },
          };
        }
        return true;
      }
      if (!authenticated && !to.meta.authentication) {
        return { name: "login", query: { redirect: to.fullPath } };
      }
      if (authenticated && to.meta.authentication) {
        const redirect = typeof to.query.redirect === "string" ? to.query.redirect : "/";
        return redirect;
      }
    }
  } catch {
    // Pages handle Core availability errors after navigation.
  }

  return true;
});
