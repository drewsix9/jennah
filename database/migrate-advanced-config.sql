-- Migration: Add advanced configuration columns to Jobs table.
-- Run these DDL statements in the Spanner console for database: labs-169405/alphaus-dev/main
--
-- The Name column already exists in the schema. Only add the remaining 5 columns.

ALTER TABLE Jobs ADD COLUMN ResourceProfile STRING(50);
ALTER TABLE Jobs ADD COLUMN MachineType STRING(255);
ALTER TABLE Jobs ADD COLUMN BootDiskSizeGb INT64;
ALTER TABLE Jobs ADD COLUMN UseSpotVms BOOL;
ALTER TABLE Jobs ADD COLUMN ServiceAccount STRING(1024);

-- Index for looking up jobs by name within a tenant.
CREATE INDEX IdxJobsByName ON Jobs(TenantId, Name);
