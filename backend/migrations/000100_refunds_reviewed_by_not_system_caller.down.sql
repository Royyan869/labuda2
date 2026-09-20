ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_reviewed_by_not_system_caller;
