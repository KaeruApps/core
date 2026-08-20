// MOCK: Kaeru Core has no backup API yet. See src/mocks/README.md.

export const backupSummary = {
  lastBackup: "August 9, 2026 at 02:00",
  lastBackupAt: "2026-08-09T02:00:00+02:00",
  nextBackup: "August 10, 2026 at 02:00",
  nextBackupAt: "2026-08-10T02:00:00+02:00",
  size: "128.4 MB",
  path: "/backups/kaeru/",
  file: "2026-08-09-kaeru-platform-backup.tar.gz",
  fileSize: "3.28 MB",
  schedule: "Every day",
  scheduledTime: "02:00",
  automatic: "Enabled",
  retention: "60",
};

export const backupScheduleOptions = [
  "Every day",
  "Every 2 days",
  "Every 3 days",
  "Every 4 days",
  "Every 5 days",
  "Every 6 days",
  "Every 7 days",
];

export const availableBackups = [
  "2026-08-09-kaeru-platform-backup.tar.gz",
  "2026-08-08-kaeru-platform-backup.tar.gz",
  "2026-08-07-kaeru-platform-backup.tar.gz",
];
