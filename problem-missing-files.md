# Missing Files Investigation Report

**Date:** January 27, 2026
**Investigated by:** Claude Code
**Issue:** User uploaded 576 files but only sees 67 in the website

---

## Executive Summary

The user uploaded 576 files (~2 hours ago) expecting to see approximately 288-443 images in the website, but only 67 appear. **Root cause: Lambda code changes were made locally but never committed or deployed.** The deployed Lambda is missing the `Status` field, causing new records to be invisible to the web UI which queries by Status.

---

## Upload Statistics

### Local Files (G:\DCIM\999_FUJI)
| File Type | Count |
|-----------|-------|
| JPG | 443 |
| RAF (RAW) | 131 |
| MOV (Video) | 2 |
| **Total** | **576** |

### S3 Bucket Status (kill-snap)

| Location | Count | Notes |
|----------|-------|-------|
| images/ (today) | 1,616 files | ~482 unique images (JPG + thumbnails + RAFs) |
| images/ (total) | 25,978 files | All processed images |
| incoming/ | 10,934 files | **STALE** - old cruft from Dec 7, 2025 |
| incoming/ (today) | 2 files | MOV files only (videos not processed) |
| corrupted/ | 0 files | No corrupted files detected |

---

## DynamoDB Statistics (kill-snap-ImageMetadata)

| Metric | Count |
|--------|-------|
| Total records | 4,747 |
| Records from today (Jan 27) | 442 |
| Records missing Status attribute | **749** |
| Records with Status="inbox" | 2,780 |
| Records with Status="deleted" | 1,104 |
| Records with Status="rejected" | 1 |
| Records with Status="approved" | 0 |
| Records with other/unknown Status | 113 |

### Critical Finding: All 442 Records From Today Are Missing the Status Attribute

```
Records from today:     442
Records missing Status: 442 (100% of today's uploads)
```

---

## Root Cause Analysis

### Primary Issue: Undeployed Lambda Code

The `ImageThumbnailGenerator` Lambda function was last deployed on **January 18, 2026 at 19:39:06 UTC**.

The deployed code does NOT include:
1. `Status` field in the `ImageMetadata` struct
2. Setting `Status: "inbox"` when creating new records

**Local changes exist but were NEVER COMMITTED OR DEPLOYED:**

```diff
# Uncommitted changes in lambda/thumbnail/main.go

+	Status           string            `json:"Status"`

    metadata := ImageMetadata{
        ...
+       Status:           "inbox",
        InsertedDateTime: now,
    }
```

### Why Users Only See 67 Images

1. The web API uses the `StatusIndex` GSI (Global Secondary Index) to query images
2. The GSI only indexes records that HAVE a `Status` attribute
3. Records without `Status` don't appear in the index
4. **Only 67 images** have `Status="inbox"` AND `OriginalFile` starting with `images/` AND are visible to the current filters

### Historical Context

| Date | Event |
|------|-------|
| Jan 18, 2026 | Lambda deployed (without Status field) |
| Jan 19, 2026 | Bulk update ran (990 records at 09:26:28-05:00) - added Status to existing records |
| Jan 22, 2026 | Previous upload batch - files processed but missing Status |
| Jan 27, 2026 | Current upload - 442 records created without Status |

---

## SQS Queue Status (us-east-2)

| Queue | Messages | Notes |
|-------|----------|-------|
| kill-snap-image-processing | 0 | All messages processed |
| kill-snap-image-processing-dlq | **339** | Failed RAW file processing from Jan 22 |

### DLQ Analysis
All 339 DLQ messages are RAF (RAW) files from January 22:
- Pattern: `incoming/2026-01-22.upload.DSCF*.RAF`
- Cause: RAW files waiting for corresponding JPG to be processed first
- These exceeded the retry limit (10 attempts)

---

## Data Inconsistencies Found

### 1. Status/Path Mismatch (391 records)
Records with `Status="inbox"` but `OriginalFile` pointing to `deleted/` path:
```
Count: 391 records
Example: Status="inbox", OriginalFile="deleted/2024/02/02/08210003.JPG"
```

### 2. Unknown Status Values (113 records)
Records with Status values other than: inbox, deleted, rejected, approved

### 3. Stale Files in incoming/ (10,934 files)
Old processed thumbnails from December 7, 2025 that should have been cleaned up:
```
incoming/001505550001.400.jpg
incoming/001505550001.50.jpg
...
```

---

## Infrastructure Details

### Region Configuration
- S3 Bucket: us-east-1 (kill-snap)
- Lambda Functions: **us-east-2**
- DynamoDB Tables: **us-east-2**
- SQS Queues: **us-east-2**

### Lambda Functions (us-east-2)
| Function | Last Modified | Status |
|----------|---------------|--------|
| ImageThumbnailGenerator | Jan 18, 2026 19:39:06 UTC | **OUT OF DATE** |
| ImageReviewApi | Jan 18, 2026 19:39:18 UTC | Active |
| ProjectZipGenerator | Jan 18, 2026 19:39:07 UTC | Active |
| DynamoDBSyncFunction | Jan 18, 2026 19:39:06 UTC | Active |

---

## Files With Uncommitted Changes

```
M lambda/api/main.go
M lambda/thumbnail/main.go
M template.yaml
M claude.md
? scripts/backfill_status.go
```

---

## Remediation Plan

### Immediate Actions (Priority 1)

1. **Commit and deploy Lambda code changes**
   - The Status field must be added to ImageThumbnailGenerator
   - Push changes via GitHub Actions pipeline (per CLAUDE.md policy)

2. **Run backfill script for missing Status**
   - Target: 749 records missing Status attribute
   - Set Status="inbox" for all records with `OriginalFile` starting with `images/`
   - Script exists: `scripts/backfill_status.go`

### Secondary Actions (Priority 2)

3. **Reprocess DLQ messages**
   - 339 RAW files need reprocessing
   - Move messages back to main queue or manually trigger

4. **Clean up stale incoming/ files**
   - 10,934 old files from Dec 7, 2025
   - These are already-processed thumbnails that weren't deleted

### Data Cleanup (Priority 3)

5. **Fix Status/Path mismatches**
   - 391 records with Status="inbox" but deleted/ paths
   - Either update Status to "deleted" or verify actual file locations

6. **Investigate unknown Status values**
   - 113 records with non-standard Status values
   - Normalize or clean up

---

## Verification Commands

### Check records missing Status
```bash
aws dynamodb scan --table-name kill-snap-ImageMetadata \
  --filter-expression "attribute_not_exists(#s)" \
  --expression-attribute-names '{"#s":"Status"}' \
  --select COUNT --region us-east-2
```

### Check today's records
```bash
aws dynamodb scan --table-name kill-snap-ImageMetadata \
  --filter-expression "begins_with(InsertedDateTime, :today)" \
  --expression-attribute-values '{":today":{"S":"2026-01-27"}}' \
  --select COUNT --region us-east-2
```

### Check DLQ depth
```bash
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-2.amazonaws.com/759775734231/kill-snap-image-processing-dlq \
  --attribute-names ApproximateNumberOfMessages \
  --region us-east-2
```

---

## Summary

| Issue | Impact | Fix |
|-------|--------|-----|
| Lambda missing Status field | 749 records invisible | Deploy updated code |
| No Status on new records | All new uploads invisible | Deploy + backfill |
| DLQ messages | 339 RAW files not linked | Reprocess queue |
| Stale incoming/ files | 10,934 files wasting storage | Clean up |
| Data inconsistencies | 504 records need cleanup | Data migration |

**Estimated affected images:** 749 (442 from today + 307 from previous uploads)

---

*Report generated: January 27, 2026*
