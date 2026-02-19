-- Migration: Add GCP Batch lifecycle tracking and environment variables
-- This migration updates the Jobs table to:
-- 1. Rename CloudJobResourcePath to GcpBatchJobName for explicit GCP naming
-- 2. Add GcpBatchTaskGroup for tracking task group status
-- 3. Add optional EnvVarsJson for storing job environment variables as JSON

-- Step 1: Add new columns to Jobs table
ALTER TABLE Jobs ADD COLUMN GcpBatchJobName STRING(1024);
ALTER TABLE Jobs ADD COLUMN GcpBatchTaskGroup STRING(1024);
ALTER TABLE Jobs ADD COLUMN EnvVarsJson STRING(MAX);

-- Step 2: Migrate existing data from CloudJobResourcePath to GcpBatchJobName
UPDATE Jobs
SET GcpBatchJobName = CloudJobResourcePath
WHERE GcpBatchJobName IS NULL AND CloudJobResourcePath IS NOT NULL;

-- Step 3: Drop old CloudJobResourcePath column (optional - uncomment when ready)
-- This should be done AFTER verifying Step 2 completed successfully and all reads use the new column
-- ALTER TABLE Jobs DROP COLUMN CloudJobResourcePath;
