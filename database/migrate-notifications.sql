-- Notifications table: persists job terminal events for in-app notification feed.
-- Populated by the consumer service from Pub/Sub; read by the gateway.
CREATE TABLE Notifications (
  TenantId       STRING(36)   NOT NULL,
  NotificationId STRING(36)   NOT NULL,
  JobId          STRING(36)   NOT NULL,
  JobName        STRING(255),
  FinalStatus    STRING(50)   NOT NULL,  -- COMPLETED | FAILED | CANCELLED
  ServiceTier    STRING(50),             -- SIMPLE | COMPLEX
  AssignedService STRING(50),            -- CLOUD_RUN_JOB | CLOUD_BATCH
  OccurredAt     TIMESTAMP    NOT NULL,
  ErrorMessage   STRING(MAX),
  IsRead         BOOL         NOT NULL DEFAULT (FALSE),
  CreatedAt      TIMESTAMP    NOT NULL OPTIONS (allow_commit_timestamp=true),
) PRIMARY KEY (TenantId, NotificationId),
  INTERLEAVE IN PARENT Tenants ON DELETE CASCADE;

CREATE INDEX NotificationsByTenant ON Notifications(TenantId, IsRead, OccurredAt DESC);
