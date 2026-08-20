<script setup>
import { onMounted, ref, watch } from "vue";
import { mdiChevronDown, mdiChevronUp } from "@mdi/js";

import defaultUserIcon from "../../assets/default-user.png";
import { useSessionStore } from "../../stores/session.js";
import { useUsersStore } from "../../stores/users.js";

const users = useUsersStore();
const session = useSessionStore();

const expandedUserId = ref(null);
const userActionToConfirm = ref(null);

onMounted(() => {
  users.load();
});

// The signed-in user appears in this list too, so reload it when they replace
// their avatar. The listing carries a versioned avatar URL, so a reload is what
// makes the new image visible here.
watch(() => session.user?.avatar_url, (next, previous) => {
  if (previous !== undefined && next !== previous) {
    users.load();
  }
});

function formatDevices(count) {
  if (!count) {
    return "No registered devices";
  }
  return `${count} registered ${count === 1 ? "device" : "devices"}`;
}

function toggleUserDetails(userId) {
  expandedUserId.value = expandedUserId.value === userId ? null : userId;
}

function requestUserAction(user, action) {
  userActionToConfirm.value = { user, action };
}

function cancelUserAction() {
  userActionToConfirm.value = null;
}

function confirmUserAction() {
  if (!userActionToConfirm.value) return;
  const { user, action } = userActionToConfirm.value;
  users.applyUnsavedUserAction(user, action);
  cancelUserAction();
}
</script>

<template>
  <section aria-labelledby="users-title" class="home-section users-section">
    <div class="home-section-header">
      <div class="home-section-heading">
        <h2 id="users-title" class="home-section-title">Users</h2>
        <p class="home-section-subtitle">
          View user info and access permissions, enable or disable users, or force user
          logout.
        </p>
      </div>
    </div>

    <p v-if="users.loading" class="service-state-message">Loading users…</p>
    <div v-else-if="users.error" class="service-state-message service-state-message--error">
      <span>{{ users.error }}</span>
      <v-btn color="primary" size="small" variant="text" @click="users.load">
        Try again
      </v-btn>
    </div>
    <p v-else-if="users.users.length === 0" class="service-state-message">
      No users have logged in yet.
    </p>
    <div v-else class="user-list">
      <v-sheet
        v-for="user in users.users"
        :key="user.id"
        class="user-list-item"
        border
        role="button"
        rounded="lg"
        tabindex="0"
        @click="toggleUserDetails(user.id)"
        @keydown.enter="toggleUserDetails(user.id)"
        @keydown.space.prevent="toggleUserDetails(user.id)"
      >
        <div class="user-list-summary">
          <div class="user-summary-identity">
            <img
              :src="user.avatarFailed ? defaultUserIcon : user.avatarUrl"
              alt=""
              class="user-avatar"
              @error="user.avatarFailed = true"
            />
            <div>
              <div class="user-name-row">
                <h3 class="user-name">{{ user.name }}</h3>
                <span v-if="user.disabled" class="user-disabled-status">
                  Disabled
                </span>
              </div>
              <p class="user-last-seen">
                {{ formatDevices(user.deviceCount) }}
              </p>
              <p class="user-last-seen">
                Last seen
                <time :datetime="user.lastSeenAt">{{ user.lastSeen }}</time>
              </p>
            </div>
          </div>
          <v-icon
            :icon="expandedUserId === user.id ? mdiChevronUp : mdiChevronDown"
            class="user-expand-icon"
          />
        </div>

        <v-expand-transition>
          <div v-show="expandedUserId === user.id" class="user-expanded-details">
            <section class="user-detail-section">
              <h4 class="user-detail-title">Registered Devices</h4>
              <p class="user-detail-description">
                Devices that {{ user.name }} has logged in on
              </p>
              <ul v-if="user.devices.length > 0" class="user-readonly-list">
                <li v-for="device in user.devices" :key="device">
                  {{ device }}
                </li>
              </ul>
              <ul v-else class="user-readonly-list">
                <li>No registered devices</li>
              </ul>
            </section>

            <section class="user-detail-section">
              <h4 class="user-detail-title">Access</h4>
              <p class="user-detail-description">
                OIDC groups: {{ user.oidcGroups.join(", ") || "None" }}
              </p>
              <dl class="user-access-list">
                <div
                  v-for="access in user.access"
                  :key="access.service"
                  class="user-access-row"
                >
                  <dt>{{ access.service }}</dt>
                  <dd>{{ access.level }}</dd>
                </div>
              </dl>
            </section>

            <div class="user-actions">
              <v-btn
                :to="{ name: 'events', query: { user: user.name } }"
                class="user-event-log-button"
                variant="text"
                @click.stop
              >
                Event Log
              </v-btn>
              <div class="user-primary-actions">
                <v-btn
                  color="error"
                  variant="flat"
                  @click.stop="requestUserAction(user, 'logout')"
                >
                  Force logout
                </v-btn>
                <v-btn
                  :color="user.disabled ? 'primary' : 'error'"
                  variant="flat"
                  @click.stop="requestUserAction(user, user.disabled ? 'enable' : 'disable')"
                >
                  {{ user.disabled ? "Enable user" : "Disable user" }}
                </v-btn>
              </div>
            </div>
          </div>
        </v-expand-transition>
      </v-sheet>
    </div>
  </section>

  <v-dialog
    :model-value="userActionToConfirm !== null"
    max-width="480"
    @update:model-value="(open) => !open && cancelUserAction()"
  >
    <v-card v-if="userActionToConfirm" class="delete-service-dialog" rounded="lg">
      <v-card-title>
        <template v-if="userActionToConfirm.action === 'logout'">
          Force logout {{ userActionToConfirm.user.name }}?
        </template>
        <template v-else-if="userActionToConfirm.action === 'disable'">
          Disable {{ userActionToConfirm.user.name }}?
        </template>
        <template v-else>
          Enable {{ userActionToConfirm.user.name }}?
        </template>
      </v-card-title>
      <v-card-text>
        <template v-if="userActionToConfirm.action === 'logout'">
          The user will be required to log in again on all devices. This can
          be used to ensure user access is updated.
        </template>
        <template v-else-if="userActionToConfirm.action === 'disable'">
          The user will be logged out on all devices and not allowed to log
          in anymore unless re-enabled.
        </template>
        <template v-else>
          The user will be able to log in on their devices again.
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="cancelUserAction">Cancel</v-btn>
        <v-btn
          :color="userActionToConfirm.action === 'enable' ? 'primary' : 'error'"
          variant="flat"
          @click="confirmUserAction"
        >
          Confirm
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<style scoped>
.users-section {
  margin-top: 48px;
}

.user-list {
  display: grid;
  gap: 12px;
}

.user-list-item {
  display: block;
  min-height: 84px;
  padding: 0;
  background: color-mix(
    in srgb,
    rgb(var(--v-theme-surface)) 75%,
    rgb(var(--v-theme-background))
  );
  cursor: pointer;
  user-select: none;
  transition: background-color 150ms ease, border-color 150ms ease;
}

.user-list-item:hover,
.user-list-item:focus-visible {
  background: rgb(var(--v-theme-surface));
  border-color: rgba(var(--v-theme-on-surface), 0.16);
}

.user-list-item:focus-visible {
  outline: 2px solid rgb(var(--v-theme-primary));
  outline-offset: 2px;
}

.user-list-summary {
  display: flex;
  min-height: 84px;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 18px 20px 18px 24px;
}

.user-summary-identity {
  align-items: center;
  display: flex;
  gap: 16px;
  min-width: 0;
}

.user-avatar {
  border-radius: 50%;
  flex: 0 0 48px;
  height: 48px;
  object-fit: cover;
  width: 48px;
}

.user-expand-icon {
  flex: 0 0 auto;
  color: rgba(var(--v-theme-on-surface), 0.65);
}

.user-name {
  margin: 0 0 4px;
  color: rgb(var(--v-theme-on-surface));
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.4;
}

.user-name-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-bottom: 4px;
}

.user-name-row .user-name {
  margin-bottom: 0;
}

.user-disabled-status {
  padding: 2px 7px;
  color: rgb(var(--v-theme-error));
  background: rgba(var(--v-theme-error), 0.12);
  border-radius: 999px;
  font-size: 0.7rem;
  font-weight: 700;
  line-height: 1.4;
}

.user-last-seen {
  margin: 0;
  color: rgba(var(--v-theme-on-surface), 0.65);
  font-size: 0.875rem;
  line-height: 1.4;
}

.user-expanded-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 32px;
  padding: 22px 24px 24px;
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.user-detail-title {
  margin: 0 0 4px;
  color: rgb(var(--v-theme-primary));
  font-size: 0.95rem;
  font-weight: 700;
  line-height: 1.4;
}

.user-detail-description {
  margin: 0 0 14px;
  color: rgba(var(--v-theme-on-surface), 0.62);
  font-size: 0.875rem;
  line-height: 1.4;
}

.user-readonly-list {
  display: grid;
  gap: 8px;
  margin: 0;
  padding-left: 20px;
  color: rgba(var(--v-theme-on-surface), 0.86);
  font-size: 0.875rem;
}

.user-access-list {
  margin: 0;
}

.user-access-row {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(var(--v-theme-on-surface), 0.08);
  font-size: 0.875rem;
}

.user-access-row:first-child {
  padding-top: 0;
}

.user-access-row:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}

.user-access-row dt {
  color: rgb(var(--v-theme-on-surface));
}

.user-access-row dd {
  margin: 0;
  color: rgb(var(--v-theme-on-surface));
  font-weight: 600;
  text-align: right;
}

.user-actions {
  display: flex;
  grid-column: 1 / -1;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
  justify-content: space-between;
  padding-top: 14px;
  border-top: 1px solid rgba(var(--v-theme-on-surface), 0.1);
}

.user-primary-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 600px) {
  .user-expanded-details {
    grid-template-columns: 1fr;
    gap: 24px;
  }

  .user-event-log-button {
    display: none;
  }

  .user-primary-actions {
    width: 100%;
    margin-left: auto;
    justify-content: flex-end;
  }
}
</style>
