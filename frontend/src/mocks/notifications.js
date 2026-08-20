// MOCK: Kaeru Core has no notification provider API yet. See src/mocks/README.md.

import emailNotificationIcon from "../assets/notification/email.png";
import kaeruNotificationsIcon from "../assets/notification/kaeru-notifications.png";

export function createNotificationServices() {
  return [
    {
      id: "kaeru-notifications",
      name: "Kaeru Notifications",
      iconUrl: kaeruNotificationsIcon,
      enabled: false,
    },
    {
      id: "email",
      name: "Email",
      iconUrl: emailNotificationIcon,
      enabled: true,
      host: "",
      port: "",
      username: "",
      password: "",
      fromAddress: "",
    },
  ];
}

export const notificationPreferenceDefaults = {
  email: "",
  deliveryMethod: "Email",
  digestFrequency: "Daily",
  quietHoursStart: "22:00",
  quietHoursEnd: "07:00",
  minimumSeverity: "Information",
  serviceStatusAlerts: "Enabled",
  backupAlerts: "Enabled",
  securityAlerts: "Enabled",
  webhookUrl: "",
};

export const notificationDeliveryMethods = ["Email", "Push", "Email and Push"];
export const notificationDigestFrequencies = ["Immediately", "Daily", "Weekly"];
export const notificationSeverityOptions = ["Information", "Warning", "Critical"];
export const notificationToggleOptions = ["Enabled", "Disabled"];
