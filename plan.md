# Remediation Plan: Project Images & Data Recovery

**Created:** January 27, 2026
**Status:** Ready for Implementation

---

## Overview

This plan addresses four critical issues discovered during the missing files investigation:

1. **Fix the sync function** to prevent future data loss
2. **Create recovery script** to rebuild DynamoDB records from S3 files
3. **Restore deleted files** from S3 versioning
4. **Investigate and recover** Grand Canyon Moon and other affected projects

---

## Issue 1: Fix DynamoDBSyncFunction

### Problem
The nightly sync function deletes DynamoDB records when it can't find files at the `OriginalFile` path - but it doesn't account for files that have been moved to project folders.

### Impact
- ~3,592 records deleted historically
- 3,118 records deleted on Jan 19, 2026 alone

### Solution
Modify `lambda/sync/main.go` to skip project images.

### Implementation

```go
// In lambda/sync/main.go, add after line 129 (after unmarshaling record):

// Skip project images - they have paths in projects/ folder
if record.Status == "project" {
    continue
}

// Skip any record with OriginalFile pointing to projects/
if strings.HasPrefix(record.OriginalFile, "projects/") {
    continue
}
```

### Files to Modify
- `lambda/sync/main.go`

### Deployment
1. Commit changes to repository
2. Push to trigger GitHub Actions pipeline
3. Verify deployment succeeded

### Verification
```bash
# Check sync function was updated
aws lambda get-function --function-name DynamoDBSyncFunction \
  --region us-east-2 --query "Configuration.LastModified"
```

---

## Issue 2: Recovery Script for DynamoDB Records

### Problem
111 original images exist in S3 project folders but have no corresponding DynamoDB records.

### Solution
Create a script that:
1. Scans S3 project folders for original images
2. Extracts metadata (EXIF, dimensions, file size)
3. Creates DynamoDB records with correct paths and Status="project"

### Implementation

Create `scripts/recover_project_images.go`:

```go
// Key logic:
// 1. List all files in s3://kill-snap/projects/
// 2. For each original image (not .50.jpg or .400.jpg):
//    a. Check if DynamoDB record exists
//    b. If not, create new record with:
//       - ImageGUID: extracted from filename or generate new
//       - OriginalFile: current S3 path
//       - Thumbnail50/400: derive from original path
//       - Status: "project"
//       - ProjectID: lookup from projects table by S3 prefix
//       - Extract EXIF data from image
```

### Script Features
- Dry-run mode (default) to preview changes
- `-apply` flag to execute changes
- `-verbose` flag for detailed output
- Progress tracking
- Error handling with retry logic

### Files to Create
- `scripts/recover_project_images.go`

### Execution
```bash
# Dry run first
cd scripts && go run recover_project_images.go

# Apply changes
cd scripts && go run recover_project_images.go -apply
```

### Expected Results
- ~111 DynamoDB records created
- Project image counts updated
- Images visible in project views

---

## Issue 3: Restore Deleted Files from S3 Versioning

### Problem
Original images and RAW files were deleted from project folders. S3 versioning is **enabled**, so files can be recovered.

### Discovery
```
S3 Versioning: ENABLED

Delete markers found for Grand Canyon Moon:
- 2692ae01-4c79-4997-b95d-d1ea727dd41e.jpg (28MB) - deleted Jan 19 00:13:14
- 2692ae01-4c79-4997-b95d-d1ea727dd41e.raf (210MB) - deleted Jan 19 00:19:16
- 290bc498-261b-4018-93f8-fef57b02d6d1.jpg (27MB) - deleted Jan 19 00:13:04

Previous versions STILL EXIST and are recoverable!
```

### Solution
Create a script to:
1. List all delete markers in project folders
2. Remove delete markers to restore files
3. Verify restored files

### Implementation

Create `scripts/restore_deleted_files.go`:

```go
// Key logic:
// 1. List object versions in s3://kill-snap/projects/
// 2. Find all delete markers
// 3. For each delete marker:
//    a. Log the file that will be restored
//    b. Delete the delete marker (this restores the file)
// 4. Verify restoration
```

### Manual Restoration (Alternative)
For specific files, use AWS CLI:

```bash
# List delete markers
aws s3api list-object-versions --bucket kill-snap \
  --prefix "projects/grand_canyon_moon/" \
  --query "DeleteMarkers[?IsLatest==\`true\`]"

# Remove a delete marker to restore file
aws s3api delete-object --bucket kill-snap \
  --key "projects/grand_canyon_moon/2019/01/01/2692ae01-4c79-4997-b95d-d1ea727dd41e.jpg" \
  --version-id "JBW_oEkymGT19rdgVaTCkorRl7CwJeY5"
```

### Files to Create
- `scripts/restore_deleted_files.go`

### Execution Order
**IMPORTANT:** Run this BEFORE the DynamoDB recovery script so records point to existing files.

```bash
# Dry run
cd scripts && go run restore_deleted_files.go

# Apply
cd scripts && go run restore_deleted_files.go -apply
```

### Expected Results
- All deleted originals and RAW files restored
- Grand Canyon Moon: 2 JPGs + 1 RAF recovered
- Other projects: Additional files recovered

---

## Issue 4: Grand Canyon Moon Investigation

### Current State
| Item | Status |
|------|--------|
| S3 Thumbnails | Present (4 files) |
| S3 Originals | Deleted (recoverable via versioning) |
| S3 RAW files | Deleted (recoverable via versioning) |
| DynamoDB Records | Missing |
| Project Record | Exists (ImageCount=0) |

### Files to Recover

| File | Size | Version ID | Delete Marker ID |
|------|------|------------|------------------|
| 2692ae01...jpg | 28MB | uJf6rnDntcZ6pzdqvbmdst20q7oT1UjC | JBW_oEkymGT19rdgVaTCkorRl7CwJeY5 |
| 2692ae01...raf | 210MB | 1LSReFubGOaVPhXbyaMGgQDAW98k8FTS | zXrL2hl.RfCZdiQnf6NL_dxcBOyZB3fa |
| 290bc498...jpg | 27MB | EK_1_UnMcdGA3U8s9yeYKVLIW5mUXqYv | krP_G675OCGGu5cV8eICpTN6FjQV7T1J |

### Recovery Steps

1. **Restore S3 files** (Issue 3)
   ```bash
   # Restore first image
   aws s3api delete-object --bucket kill-snap \
     --key "projects/grand_canyon_moon/2019/01/01/2692ae01-4c79-4997-b95d-d1ea727dd41e.jpg" \
     --version-id "JBW_oEkymGT19rdgVaTCkorRl7CwJeY5"

   # Restore RAW file
   aws s3api delete-object --bucket kill-snap \
     --key "projects/grand_canyon_moon/2019/01/01/2692ae01-4c79-4997-b95d-d1ea727dd41e.raf" \
     --version-id "zXrL2hl.RfCZdiQnf6NL_dxcBOyZB3fa"

   # Restore second image
   aws s3api delete-object --bucket kill-snap \
     --key "projects/grand_canyon_moon/2019/01/01/290bc498-261b-4018-93f8-fef57b02d6d1.jpg" \
     --version-id "krP_G675OCGGu5cV8eICpTN6FjQV7T1J"
   ```

2. **Create DynamoDB records** (Issue 2)
   - Run recovery script after files are restored

3. **Update project image count**
   ```bash
   aws dynamodb update-item --table-name kill-snap-Projects \
     --key '{"ProjectID":{"S":"59b20b6b-bc79-418d-890b-3d1fddbc363a"}}' \
     --update-expression "SET ImageCount = :count" \
     --expression-attribute-values '{":count":{"N":"2"}}' \
     --region us-east-2
   ```

4. **Verify recovery**
   - Check website shows images in Grand Canyon Moon project
   - Verify thumbnails load correctly
   - Verify original download works

---

## Execution Order

### Phase 1: Prevent Further Damage
1. [ ] Fix sync function (`lambda/sync/main.go`)
2. [ ] Commit and deploy via GitHub Actions
3. [ ] Verify deployment

### Phase 2: Restore S3 Files
4. [ ] Create `scripts/restore_deleted_files.go`
5. [ ] Run dry-run to inventory deleted files
6. [ ] Run with `-apply` to restore files
7. [ ] Verify files restored in S3

### Phase 3: Rebuild DynamoDB Records
8. [ ] Create `scripts/recover_project_images.go`
9. [ ] Run dry-run to preview records to create
10. [ ] Run with `-apply` to create records
11. [ ] Verify records in DynamoDB

### Phase 4: Verification
12. [ ] Check Grand Canyon Moon shows images
13. [ ] Check other affected projects
14. [ ] Verify project image counts are correct
15. [ ] Test image viewing and download

---

## Affected Projects Inventory

### Projects Needing S3 File Restoration
(Have only thumbnails, originals deleted)

| Project | S3 Prefix | Thumbnails | Originals | Status |
|---------|-----------|------------|-----------|--------|
| Grand Canyon Moon | grand_canyon_moon | 4 | 0 (2 recoverable) | Needs restore |
| Charlotte 2025 | charlotte_2025 | Multiple | 0 | Check versions |
| Building Target 2025 | building_target_2025 | Multiple | 0 | Check versions |
| Family 2025 | family_2025 | Multiple | 0 | Check versions |

### Projects Needing DynamoDB Records Only
(Have originals in S3, missing DB records)

| Project | S3 Prefix | Images | DynamoDB Records |
|---------|-----------|--------|------------------|
| South West | south_west | 45 | Check needed |
| Heavy Metal Polaroids | heavy_metal_polaroids | 34 | Check needed |
| Virgin Cruise 2024 | virgin_cruise_2024 | 9 | Check needed |
| Local Landscape | local_landscape | 6 | Check needed |
| Fire & Night | fire_night | 5 | Check needed |
| Polaroid Abstracts | polaroid_abstracts | 5 | Check needed |
| Miami | miami | 4 | Check needed |
| Family | family | 2 | Check needed |
| Kitchens | kitchens | 2 | Check needed |
| Route 66 | route_66 | 1 | Check needed |

---

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Script creates duplicate records | Medium | Low | Check for existing records before insert |
| Wrong ProjectID assigned | Low | Medium | Validate against projects table |
| Restored files overwritten | Low | High | Verify before running sync again |
| Pipeline deployment fails | Low | Medium | Monitor and remediate per CLAUDE.md |

---

## Rollback Plan

### If DynamoDB records are incorrect:
```bash
# Delete records created by recovery script
# (Script should log all created GUIDs for easy rollback)
```

### If S3 files are corrupted after restore:
- S3 versioning preserves all versions
- Can restore to any previous version

---

## Success Criteria

- [ ] Sync function no longer deletes project images
- [ ] All recoverable S3 files restored
- [ ] DynamoDB records exist for all project images
- [ ] Grand Canyon Moon shows 2 images
- [ ] All project image counts accurate
- [ ] No errors in next sync function run

---

## Estimated Effort

| Task | Effort |
|------|--------|
| Fix sync function | 15 min |
| Create restore script | 30 min |
| Create recovery script | 45 min |
| Testing & verification | 30 min |
| **Total** | **~2 hours** |

---

*Plan created: January 27, 2026*
