# Automatic Remediation Plan

**Created:** January 27, 2026
**Status:** Active

---

## Overview

This document outlines automatic remediation strategies to prevent data loss and ensure system resilience for the kill-snap image management system.

---

## 1. S3 Versioning & Lifecycle Management

### Current State
- **Versioning:** Enabled ✓
- **Noncurrent Version Retention:** Indefinite (no expiration rule)

### Recommendation: Add Noncurrent Version Expiration

Add a lifecycle rule to automatically expire old versions after a retention period, balancing recovery capability with storage costs.

**Proposed Rule:**
```json
{
  "Rules": [
    {
      "ID": "ExpireOldVersions",
      "Status": "Enabled",
      "Filter": {
        "Prefix": "projects/"
      },
      "NoncurrentVersionExpiration": {
        "NoncurrentDays": 90
      }
    },
    {
      "ID": "ExpireOldImagesVersions",
      "Status": "Enabled",
      "Filter": {
        "Prefix": "images/"
      },
      "NoncurrentVersionExpiration": {
        "NoncurrentDays": 90
      }
    }
  ]
}
```

**Implementation:**
```bash
aws s3api put-bucket-lifecycle-configuration \
  --bucket kill-snap \
  --lifecycle-configuration file://lifecycle-policy.json \
  --region us-east-2
```

**Trade-off:** 90 days provides ample recovery window while controlling storage costs. Adjust based on your recovery needs.

---

## 2. DynamoDB Backup Strategy

### Current State
- No automated backups configured
- Point-in-time recovery: Unknown

### Recommendation: Enable Point-in-Time Recovery (PITR)

PITR allows recovery to any point within the last 35 days.

**Implementation:**
```bash
# Enable PITR for ImageMetadata table
aws dynamodb update-continuous-backups \
  --table-name kill-snap-ImageMetadata \
  --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true \
  --region us-east-2

# Enable PITR for Projects table
aws dynamodb update-continuous-backups \
  --table-name kill-snap-Projects \
  --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true \
  --region us-east-2
```

**Cost:** ~$0.20 per GB-month for backup storage

---

## 3. Sync Function Safety Improvements

### Current State
- Fixed to skip project images ✓
- No dry-run capability
- No audit logging

### Recommendation: Add Safety Features

#### A. Create CloudWatch Alarm for Mass Deletions

```yaml
# Add to template.yaml
SyncFunctionHighDeletionAlarm:
  Type: AWS::CloudWatch::Alarm
  Properties:
    AlarmName: kill-snap-sync-high-deletions
    AlarmDescription: Alert when sync function deletes more than 50 records
    MetricName: OrphansRemoved
    Namespace: KillSnap/Sync
    Statistic: Sum
    Period: 86400  # 24 hours
    EvaluationPeriods: 1
    Threshold: 50
    ComparisonOperator: GreaterThanThreshold
    AlarmActions:
      - !Ref AlertSNSTopic
```

#### B. Add Deletion Threshold to Sync Function

Modify `lambda/sync/main.go` to abort if deletions exceed a threshold:

```go
const maxDeletionsPerRun = 100

// Before deleting, check threshold
if result.OrphansRemoved >= maxDeletionsPerRun {
    fmt.Printf("WARNING: Deletion threshold reached (%d). Aborting to prevent mass deletion.\n", maxDeletionsPerRun)
    result.Errors = append(result.Errors, "Deletion threshold reached - manual review required")
    break
}
```

#### C. Add Custom Metrics

```go
// Publish metrics to CloudWatch
cloudwatch.PutMetricData(&cloudwatch.PutMetricDataInput{
    Namespace: aws.String("KillSnap/Sync"),
    MetricData: []*cloudwatch.MetricDatum{
        {
            MetricName: aws.String("OrphansRemoved"),
            Value:      aws.Float64(float64(result.OrphansRemoved)),
            Unit:       aws.String("Count"),
        },
        {
            MetricName: aws.String("TotalScanned"),
            Value:      aws.Float64(float64(result.TotalScanned)),
            Unit:       aws.String("Count"),
        },
    },
})
```

---

## 4. Automated Health Checks

### Recommendation: Create Health Check Lambda

A scheduled Lambda that runs daily to detect anomalies:

```go
// health_check.go - Detects data inconsistencies

func healthCheck() HealthReport {
    report := HealthReport{}

    // Check 1: Images without Status field
    report.MissingStatus = countImagesWithoutStatus()

    // Check 2: Project images without thumbnails
    report.MissingThumbnails = countProjectImagesWithoutThumbnails()

    // Check 3: S3 files without DynamoDB records
    report.OrphanS3Files = countOrphanS3Files()

    // Check 4: DynamoDB records without S3 files (excluding projects)
    report.OrphanDBRecords = countOrphanDBRecords()

    // Check 5: Projects with incorrect ImageCount
    report.IncorrectCounts = checkProjectCounts()

    // Alert if any issues found
    if report.HasIssues() {
        sendAlertEmail(report)
    }

    return report
}
```

**Schedule:** Daily at 3 AM UTC via EventBridge

---

## 5. Recovery Scripts Inventory

The following scripts are available in `/scripts/` for manual recovery:

| Script | Purpose | Usage |
|--------|---------|-------|
| `restore_deleted_files.go` | Restore S3 files from versioning | `go run restore_deleted_files.go -apply` |
| `recover_project_images.go` | Rebuild DynamoDB records from S3 | `go run recover_project_images.go -apply` |
| `fix_thumbnail_paths.go` | Fix NULL thumbnail paths | `go run fix_thumbnail_paths.go -apply` |
| `regenerate_thumbnails.go` | Regenerate missing thumbnails | `go run regenerate_thumbnails.go -apply` |
| `update_project_counts.go` | Sync project ImageCount values | `go run update_project_counts.go -apply` |
| `cleanup_test_folders.go` | Remove orphan test folders | `go run cleanup_test_folders.go -apply` |
| `backfill_status.go` | Add Status field to old records | `go run backfill_status.go -apply` |

**All scripts support dry-run mode by default.** Add `-apply` flag to execute changes.

---

## 6. Monitoring Dashboard

### Recommendation: Create CloudWatch Dashboard

```yaml
# dashboard.yaml
Widgets:
  - Type: metric
    Title: "Sync Function Activity"
    Metrics:
      - KillSnap/Sync OrphansRemoved
      - KillSnap/Sync TotalScanned
      - KillSnap/Sync Errors

  - Type: metric
    Title: "DynamoDB Health"
    Metrics:
      - AWS/DynamoDB ConsumedReadCapacityUnits
      - AWS/DynamoDB ConsumedWriteCapacityUnits

  - Type: metric
    Title: "S3 Activity"
    Metrics:
      - AWS/S3 NumberOfObjects
      - AWS/S3 BucketSizeBytes
```

---

## 7. Incident Response Runbook

### Scenario: Mass Deletion Detected

1. **Immediate Actions:**
   - Disable the sync function: `aws lambda update-function-configuration --function-name DynamoDBSyncFunction --environment "Variables={DISABLED=true}"`
   - Check CloudWatch logs for scope of deletion

2. **Assessment:**
   ```bash
   # Check recent sync runs
   aws logs filter-log-events \
     --log-group-name /aws/lambda/DynamoDBSyncFunction \
     --start-time $(date -d '24 hours ago' +%s)000 \
     --filter-pattern "Orphan"
   ```

3. **Recovery:**
   ```bash
   cd scripts
   # Restore S3 files
   go run restore_deleted_files.go -apply
   # Rebuild DynamoDB records
   go run recover_project_images.go -apply
   # Fix thumbnail paths
   go run fix_thumbnail_paths.go -apply
   ```

4. **Verification:**
   ```bash
   # Check project image counts
   go run update_project_counts.go
   ```

5. **Root Cause Analysis:**
   - Review what triggered the false positives
   - Update sync function logic if needed
   - Re-enable sync function after fix is deployed

---

## 8. Implementation Priority

| Priority | Task | Effort | Impact |
|----------|------|--------|--------|
| 1 | Enable DynamoDB PITR | 5 min | High |
| 2 | Add sync deletion threshold | 30 min | High |
| 3 | Add S3 lifecycle for old versions | 10 min | Medium |
| 4 | Create CloudWatch alarms | 30 min | Medium |
| 5 | Build health check Lambda | 2 hours | Medium |
| 6 | Create monitoring dashboard | 1 hour | Low |

---

## 9. Maintenance Schedule

| Task | Frequency | Script/Method |
|------|-----------|---------------|
| Review sync function logs | Weekly | CloudWatch Logs |
| Check project image counts | Monthly | `update_project_counts.go` |
| Clean orphan test folders | Monthly | `cleanup_test_folders.go` |
| Review S3 storage costs | Monthly | AWS Cost Explorer |
| Test recovery procedures | Quarterly | Manual drill |

---

## 10. Quick Reference Commands

```bash
# Check for orphan DynamoDB records
aws dynamodb scan --table-name kill-snap-ImageMetadata \
  --filter-expression "attribute_not_exists(#status)" \
  --expression-attribute-names '{"#status":"Status"}' \
  --select COUNT --region us-east-2

# Count project images
aws dynamodb scan --table-name kill-snap-ImageMetadata \
  --filter-expression "#status = :project" \
  --expression-attribute-names '{"#status":"Status"}' \
  --expression-attribute-values '{":project":{"S":"project"}}' \
  --select COUNT --region us-east-2

# List delete markers in projects/
aws s3api list-object-versions --bucket kill-snap \
  --prefix "projects/" \
  --query "DeleteMarkers[?IsLatest==\`true\`].Key" \
  --region us-east-2

# Check DynamoDB PITR status
aws dynamodb describe-continuous-backups \
  --table-name kill-snap-ImageMetadata \
  --region us-east-2
```

---

*Last Updated: January 27, 2026*
