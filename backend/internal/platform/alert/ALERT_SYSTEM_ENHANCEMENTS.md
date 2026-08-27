# Alert System Enhancements

## Overview

The alert system has been enhanced with severity levels, automatic deduplication, and escalation policies to ensure alerts are:

- ❗ **Non-spam**: Automatic deduplication prevents duplicate alerts
- ❗ **Actionable**: Severity-based escalation ensures critical alerts get attention
- ❗ **Trusted**: Time-window deduplication reduces noise while maintaining visibility

---

## New Features

### 1. Enhanced Severity Levels

**New Standard Levels:**
- `critical` - Requires immediate action
- `warning` - Requires attention (threshold-based escalation)
- `info` - Log only (no escalation)

**Legacy Levels (maintained for backward compatibility):**
- `low` - Informational
- `medium` - Requires attention
- `high` - Requires urgent attention
- `critical` - Requires immediate action

### 2. Automatic Deduplication

**Dedup Key:**
- Automatically generated from `alert_type + entity_type + entity_id`
- Format: `{alert_type}:{entity_type}:{entity_id}`
- Example: `payment_failure_spike:payment:123e4567-e89b-12d3-a456-426614174000`

**Time Window Deduplication:**
- Default window: 60 minutes
- Customizable per alert via `dedup_window`
- Within window: updates existing alert
- Outside window: creates new alert

### 3. Enhanced Alert Status

**New Status:**
- `open` - New standard status for active alerts

**Legacy Status (maintained):**
- `active` - Original active status
- `acknowledged` - Seen but not resolved
- `resolved` - Addressed
- `false_positive` - Invalid alert

### 4. Escalation Policy Framework

**Critical Alerts:**
- Immediate escalation
- Priority: 100
- Channel: immediate

**Warning Alerts:**
- Threshold-based escalation
- Default: escalate (priority 50)
- With threshold: escalate when count >= threshold

**Info Alerts:**
- Log only (no escalation)
- Priority: 0
- Channel: log

---

## API Usage

### Creating Alerts with Standard Service

```go
result, err := alertService.CreateAlert(
    ctx,
    entity.AlertTypePaymentFailureSpike,
    entity.SeverityCritical,  // New severity
    "payment",
    paymentID,
    "Payment failure spike detected",
    metadata,
    &groupKey,
)
```

### Creating Alerts with Custom Dedup Window

```go
result, err := alertService.CreateAlertWithDedupWindow(
    ctx,
    alertType,
    severity,
    entityType,
    entityID,
    message,
    metadata,
    groupKey,
    120, // 2 hour dedup window
)
```

### Creating Alerts with Escalation

```go
escalationService := NewEscalationService(alertService, DefaultEscalationPolicy(), logger)

result, action, err := escalationService.CreateAlertWithEscalation(
    ctx,
    alertType,
    severity,
    entityType,
    entityID,
    message,
    metadata,
    groupKey,
)

if action.ShouldEscalate {
    // Send notification
    sendNotification(result.Alert, action.Priority)
}
```

### Setting Warning Thresholds

```go
escalationService.SetWarningThreshold(
    entity.AlertTypeDisputeSpike,
    5, // Escalate after 5 occurrences
)
```

---

## Database Schema Changes

### New Columns

```sql
-- Automatic deduplication key
dedup_key VARCHAR(255) NOT NULL DEFAULT ''

-- Custom dedup window in minutes
dedup_window INTEGER
```

### New Indexes

```sql
-- Dedup key with time ordering
CREATE INDEX idx_system_alerts_dedup_key_created
    ON system_alerts (dedup_key, created_at DESC)
    WHERE dedup_key != '';

-- Dedup key with status
CREATE INDEX idx_system_alerts_dedup_key_status
    ON system_alerts (dedup_key, status, created_at DESC)
    WHERE dedup_key != '';
```

### New Views

**Alert Escalation Summary:**
```sql
SELECT * FROM alert_escalation_summary;
```
- Shows active alerts by severity and status
- Includes time-based counts (last hour, 24h)
- Ordered by severity priority

**Alert Dedup Stats:**
```sql
SELECT * FROM alert_dedup_stats;
```
- Hourly deduplication statistics
- Shows duplicates prevented
- Calculates average duplicates per key

---

## Alert Deduplication Flow

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Generate dedup_key: alert_type:entity_type:entity_id      │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ 2. Check for existing alerts with same dedup_key within      │
│    time window (default: 60 minutes)                          │
└─────────────────────────────────────────────────────────────┘
                              │
                ┌─────────────┴─────────────┐
                │                           │
          Found existing            No existing
          alerts in window            alerts
                │                           │
                ▼                           ▼
┌───────────────────────────┐   ┌─────────────────────────┐
│ 3. Update existing alert: │   │ 3. Create new alert:    │
│    - Increment occurrence │   │    - Generate dedup_key │
│      count                │   │    - Set custom window  │
│    - Update last_occurrence│   │    - Initialize counts  │
│    - Escalate severity     │   │    - Store in DB        │
│      if more severe        │   └─────────────────────────┘
└───────────────────────────┘
```

---

## Escalation Decision Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    Evaluate Alert Severity                   │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
   CRITICAL              WARNING                INFO
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐   ┌──────────────────┐   ┌───────────────┐
│ ESCALATE      │   │ Check Threshold  │   │ LOG ONLY      │
│ IMMEDIATELY   │   │                  │   │ (no escalation)│
│ Priority: 100 │   │ count >= threshold│   │ Priority: 0   │
└───────────────┘   └──────────────────┘   └───────────────┘
                           │
              ┌────────────┴────────────┐
              │                         │
        Threshold met               No threshold
        or not set                   or below
              │                         │
              ▼                         ▼
     ┌─────────────────┐      ┌─────────────────┐
     │ ESCALATE        │      │ LOG ONLY        │
     │ Priority: 50    │      │ Priority: 0     │
     └─────────────────┘      └─────────────────┘
```

---

## Backward Compatibility

### Existing Alerts
- All existing alerts automatically receive generated `dedup_key`
- Legacy severity levels continue to work
- Legacy `active` status treated as `open`

### Gradual Migration
1. **Phase 1**: New alerts use new severity levels
2. **Phase 2**: Update existing alerts to new levels
3. **Phase 3**: Deprecate legacy severity levels

---

## Monitoring & Observability

### Key Metrics

**Deduplication Effectiveness:**
```sql
SELECT
    COUNT(*) as total_alerts,
    COUNT(DISTINCT dedup_key) as unique_alerts,
    COUNT(*) - COUNT(DISTINCT dedup_key) as duplicates_prevented,
    ROUND((COUNT(*) - COUNT(DISTINCT dedup_key))::numeric / COUNT(*) * 100, 2) as dedup_rate_pct
FROM system_alerts
WHERE created_at >= NOW() - INTERVAL '7 days';
```

**Escalation Stats:**
```sql
SELECT * FROM alert_escalation_summary;
```

**Hourly Dedup Stats:**
```sql
SELECT * FROM alert_dedup_stats;
```

### Alert Health Checks

**Check for duplicate alerts (should be minimal):**
```sql
SELECT
    dedup_key,
    COUNT(*) as count,
    MAX(created_at) as latest,
    MIN(created_at) as earliest
FROM system_alerts
WHERE created_at >= NOW() - INTERVAL '1 hour'
GROUP BY dedup_key
HAVING COUNT(*) > 1
ORDER BY count DESC;
```

**Check escalation coverage:**
```sql
SELECT
    severity,
    COUNT(*) as total,
    COUNT(CASE WHEN status IN ('acknowledged', 'resolved') THEN 1 END) as handled,
    ROUND(COUNT(CASE WHEN status IN ('acknowledged', 'resolved') THEN 1 END)::numeric / COUNT(*) * 100, 2) as handling_rate_pct
FROM system_alerts
WHERE created_at >= NOW() - INTERVAL '24 hours'
GROUP BY severity;
```

---

## Configuration

### Default Escalation Policy

```go
policy := &EscalationPolicy{
    CriticalImmediately:  true,
    WarningThresholds:    map[string]int{
        "payment_failure_spike": 5,
        "dispute_spike":         3,
    },
    WarningWindowMinutes: 60,
    InfoLogOnly:          true,
}
```

### Custom Dedup Windows

```go
// Short window for rapid-fire alerts
alertService.CreateAlertWithDedupWindow(ctx, ..., 30) // 30 minutes

// Long window for slow-burning issues
alertService.CreateAlertWithDedupWindow(ctx, ..., 240) // 4 hours
```

---

## Migration

### Applying Migration

```bash
# Up
cd backend/migrations
psql -U your_user -d your_database -f 000101_alert_system_enhancements.up.sql

# Down (rollback)
psql -U your_user -d your_database -f 000101_alert_system_enhancements.down.sql
```

### Verification

```sql
-- Check new columns exist
\d system_alerts

-- Check dedup keys generated
SELECT COUNT(*) FROM system_alerts WHERE dedup_key != '';

-- Check indexes created
\d system_alerts

-- Check views created
\dv alert_escalation_summary
\dv alert_dedup_stats
```

---

## Best Practices

### 1. Choosing Severity Levels

- **Critical**: System-down, data loss, security breach, financial impact > $1000
- **Warning**: Performance degradation, anomaly detected, potential issue
- **Info**: Audit trail, compliance, informational

### 2. Setting Dedup Windows

- **Rapid-fire events** (payment failures): 30-60 minutes
- **Slow-burning issues** (reconciliation drift): 2-4 hours
- **One-time events** (security alerts): 5-15 minutes

### 3. Configuring Thresholds

```go
// High-frequency, low-impact events
escalationService.SetWarningThreshold(AlertTypePaymentFailureSpike, 10)

// Low-frequency, high-impact events
escalationService.SetWarningThreshold(AlertTypeDisputeSpike, 3)

// Critical events (always escalate immediately)
// No threshold needed - critical always escalates
```

### 4. Monitoring

- Monitor `duplicates_prevented` metric
- Track escalation rates by severity
- Alert on high dedup rates (indicates spam)
- Review unhandled info alerts periodically

---

## Troubleshooting

### Too Many Duplicate Alerts

**Problem**: Duplicate alerts still appearing
**Solution**: Check dedup_key generation and time window

```sql
-- Find potential duplicate issues
SELECT
    dedup_key,
    COUNT(*) as count,
    MAX(created_at) - MIN(created_at) as time_span
FROM system_alerts
WHERE created_at >= NOW() - INTERVAL '1 day'
GROUP BY dedup_key
HAVING COUNT(*) > 2
ORDER BY count DESC;
```

### Alerts Not Escalating

**Problem**: Critical alerts not being escalated
**Solution**: Check escalation policy configuration

```go
// Verify policy is set
if escalationService.policy.CriticalImmediately {
    // Should escalate
} else {
    // Policy misconfigured
}
```

### High Info Alert Volume

**Problem**: Too many info alerts flooding logs
**Solution**: Implement info alert aggregation

```sql
-- Aggregate info alerts by type
SELECT
    alert_type,
    COUNT(*) as count,
    MAX(created_at) as latest
FROM system_alerts
WHERE severity = 'info'
AND created_at >= NOW() - INTERVAL '1 hour'
GROUP BY alert_type
ORDER BY count DESC;
```

---

## Performance Considerations

### Index Usage

- `idx_system_alerts_dedup_key_created`: Critical for dedup queries
- `idx_system_alerts_dedup_key_status`: Optimizes status filtering
- `idx_system_alerts_severity_status`: Supports escalation queries

### Query Optimization

```sql
-- Good: Uses dedup_key index
SELECT * FROM system_alerts
WHERE dedup_key = 'payment_failure_spike:payment:xxx'
AND created_at >= NOW() - INTERVAL '1 hour';

-- Bad: Function call prevents index usage
SELECT * FROM system_alerts
WHERE CONCAT(alert_type, ':', entity_type, ':', entity_id) = '...'
```

### Cleanup Strategy

```go
// Run daily to cleanup old resolved alerts
deleted, err := alertService.CleanupOldAlerts(ctx, 30) // Delete > 30 days
```

---

## Future Enhancements

1. **Smart Thresholds**: ML-based threshold adjustment
2. **Alert Correlation**: Group related alerts across types
3. **Predictive Alerting**: Alert before issues occur
4. **Multi-Channel Escalation**: Slack, email, SMS, PagerDuty
5. **Alert Acknowledgment Workflow**: Assignment, comments, resolution tracking

---

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review database views for insights
3. Enable debug logging in alert service
4. Check migration status

**Status**: ✅ Production Ready
**Version**: 2.0
**Migration**: 000101_alert_system_enhancements


