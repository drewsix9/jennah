-- Migration: Add Reason column to JobStateTransitions table
-- This column stores the reason for state transitions (e.g., error details, cancellation reason)

ALTER TABLE JobStateTransitions
ADD COLUMN Reason STRING(MAX);
