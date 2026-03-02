-- Migration: Rename GcpBatchJobName to GcpBatchJobPath
--
-- This migration renames the GcpBatchJobName column to GcpBatchJobPath in the
-- Jobs table. Cloud Spanner does not support ALTER TABLE ... RENAME COLUMN,
-- so the migration is performed in three steps:
--
--   Step 1: Add the new column
--   Step 2: Copy data from old column to new column (run via DML)
--   Step 3: Drop the old column
--
-- Execute each step separately and verify before proceeding to the next.

-- Step 1: Add the new column.
ALTER TABLE Jobs ADD COLUMN GcpBatchJobPath STRING(1024);

-- Step 2: Copy data from old column to new column.
-- Run this as a DML statement (not DDL):
--   UPDATE Jobs SET GcpBatchJobPath = GcpBatchJobName WHERE TRUE;

-- Step 3: Drop the old column (only after verifying Step 2 completed).
-- ALTER TABLE Jobs DROP COLUMN GcpBatchJobName;
