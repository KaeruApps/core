import { createApp } from "vue";
import { createVuetify } from "vuetify";
import {
  VApp,
  VAppBar,
  VAppBarTitle,
  VAutocomplete,
  VBtn,
  VCard,
  VCardActions,
  VCardText,
  VCardTitle,
  VCheckbox,
  VContainer,
  VDialog,
  VExpandTransition,
  VFileInput,
  VIcon,
  VList,
  VListItem,
  VMain,
  VMenu,
  VSheet,
  VSpacer,
  VSelect,
  VSwitch,
  VTextField,
} from "vuetify/components";
import { aliases, mdi } from "vuetify/iconsets/mdi-svg";
import { Ripple } from "vuetify/directives";
import "@fontsource-variable/roboto-condensed";
import "vuetify/styles";

import App from "./App.vue";
import { router } from "./router.js";
import "./style.css";

const vuetify = createVuetify({
  directives: {
    Ripple,
  },
  components: {
    VApp,
    VAppBar,
    VAppBarTitle,
    VAutocomplete,
    VBtn,
    VCard,
    VCardActions,
    VCardText,
    VCardTitle,
    VCheckbox,
    VContainer,
    VDialog,
    VExpandTransition,
    VFileInput,
    VIcon,
    VList,
    VListItem,
    VMain,
    VMenu,
    VSheet,
    VSpacer,
    VSelect,
    VSwitch,
    VTextField,
  },
  icons: {
    defaultSet: "mdi",
    aliases,
    sets: {
      mdi,
    },
  },
  theme: {
    defaultTheme: "kaeruDark",
    themes: {
      kaeruDark: {
        dark: true,
        colors: {
          background: "#121416",
          surface: "#1e2124",
          appbar: "#272a2d",
          primary: "#439961",
          settings: "#429961",
          error: "#EF5350",
          warning: "#D9A514",
        },
      },
      kaeruLight: {
        dark: false,
        colors: {
          background: "#F4F6F5",
          surface: "#FFFFFF",
          appbar: "#FFFFFF",
          primary: "#439961",
          settings: "#439961",
          error: "#C62828",
          warning: "#8A6500",
        },
      },
    },
  },
});

createApp(App).use(router).use(vuetify).mount("#app");
