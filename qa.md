# Kill-Snap QA & Remediation Guide

## Test Suite

### Running Tests

```bash
# Run all tests
npx playwright test --reporter=list

# Run a specific test by name
npx playwright test --grep "5. Full zip" --reporter=list

# Run with headed browser for debugging
npx playwright test --headed --grep "1. Login"
```

**Config**: `playwright.config.ts` - baseURL: `https://kill-snap.jeremy.ninja`, timeout: 120s per test, 180s test-level timeout.

### Test Inventory

| # | Test Name | What It Validates |
|---|-----------|-------------------|
| 1 | Login and inventory current state | Auth flow, image counts by state, project listing |
| 2 | Group assignment + add to project | Approve images with group, add to project via API and UI |
| 3 | Single image add to project via modal | Add a single image to a project by GUID |
| 4 | Test create zip functionality | Trigger zip generation on an existing project, poll to completion |
| 5 | Full zip generation test | **End-to-end**: approve images -> add to project -> generate zip -> validate download |
| 6 | UI integration - gallery add to project flow | UI-driven workflow: filter by state/group, select project, click Add to Project |

### Test Prerequisites

- At least 2 **unreviewed** images must exist for test 5 (it approves fresh images each run)
- Tests create temporary projects (named `Zip Test {timestamp}`) that accumulate over time
- Screenshots are saved to `tests/screenshots/` but wrapped in try/catch (non-blocking)

### Test Architecture

Tests use `page.evaluate()` to make API calls directly from the browser context, reusing the auth token from `localStorage`. This approach:
- Matches production auth flow (same headers, same cookies)
- Avoids CORS issues that affect direct `fetch()` from the test runner
- Works reliably in headless mode

---

## Image Lifecycle

Understanding the image lifecycle is critical for debugging failures at any stage.

```
Upload to S3 inbox/
       |
       v
[Thumbnail Generator Lambda] (triggered by S3 event via SQS)
  - Copies original to images/{UUID}.jpg
  - Creates images/{UUID}.50.jpg (50px thumbnail)
  - Creates images/{UUID}.400.jpg (400px thumbnail)
  - Creates DynamoDB record (Status=inbox, Reviewed=false)
  - Deletes original incoming file
       |
       v
User approves image (PUT /api/images/{id})
  - Sets GroupNumber, ColorCode, Reviewed=true
  - Sets MoveStatus=pending
  - Triggers async Lambda self-invocation
       |
       v
[Async Move Lambda] (InvocationType=Event)
  - Copies files from images/ to approved/{color}/YYYY/MM/DD/
  - Deletes files from images/
  - Updates DynamoDB: Status=approved, MoveStatus=complete, new file paths
       |
       v
Add to Project (POST /api/projects/{id}/images)
  - Queries approved images by group (GroupStatusIndex) or all (StatusIndex)
  - Also includes inbox images with Reviewed=true (pending async move)
  - Copies files from approved/{color}/ to projects/{name}/YYYY/MM/DD/
  - Updates DynamoDB: Status=project, ProjectID set
       |
       v
Generate Zip (POST /api/projects/{id}/generate-zip)
  - Queries ProjectIndex for all images in project
  - Downloads each image from S3
  - Creates zip with STORE mode (no compression for JPEGs)
  - Embeds XMP metadata for JPEGs
  - Uploads zip to project-zips/
```

### S3 Prefix Organization

| Prefix | Purpose | Lifecycle Policy |
|--------|---------|-----------------|
| `images/` | Processed originals + thumbnails (pre-review) | None |
| `approved/{color}/YYYY/MM/DD/` | Approved images organized by group color | None |
| `projects/{name}/YYYY/MM/DD/` | Images assigned to a project | None |
| `deleted/` | Soft-deleted images | 90-day expiration |
| `rejected/` | Rejected images | 365-day expiration |
| `project-zips/` | Generated zip files | 30-day expiration |
| `corrupted/` | Files that failed processing | 90-day expiration |

### S3 Event Notifications

S3 bucket notifications trigger on **all** `s3:ObjectCreated:*` events for `.jpg`, `.jpeg`, `.JPG`, `.JPEG`, and RAW file extensions (`.cr2`, `.CR2`, `.nef`, `.NEF`, etc.). There is **no prefix filter** on notifications. The thumbnail generator Lambda skips files in already-organized prefixes (`images/`, `approved/`, `projects/`, `deleted/`, `rejected/`, `corrupted/`, `project-zips/`).

---

## Bugs Found & Remediated

### Bug 1: Thumbnail Generator Deleting Moved Files

**Commit**: `78921fb`
**Severity**: Critical - prevented all add-to-project and zip generation from working

**Root Cause**: When the async move Lambda copies a `.jpg` file from `images/` to `approved/{color}/...` using S3 CopyObject, the `ObjectCreated` event triggers the thumbnail generator Lambda via SQS. The thumbnail generator's idempotency check (`checkIdempotency`) finds the existing DynamoDB record for the image GUID and treats the copied file as a duplicate upload, then **deletes it** via `deleteOriginalFile()`. Thumbnails (`.50.jpg`, `.400.jpg`) survived because the handler skips files containing `.50.` or `.400.` in the key.

**Symptoms**:
- `movedCount: 0` for all `addToProject` calls despite approved images existing in DynamoDB
- Zip generation produced empty 22-byte files (EOCD record only, 0 images)
- DynamoDB showed `Status=approved`, `MoveStatus=complete` with correct paths, but S3 HeadObject returned 404
- Thumbnails existed at the approved path, but original `.jpg` was missing

**Fix**: Added `approved/` and `projects/` to the skip list in `lambda/thumbnail/main.go`:
```go
if strings.HasPrefix(key, "deleted/") || strings.HasPrefix(key, "rejected/") ||
    strings.HasPrefix(key, "corrupted/") || strings.HasPrefix(key, "project-zips/") ||
    strings.HasPrefix(key, "approved/") || strings.HasPrefix(key, "projects/") {
    continue
}
```

**How it was diagnosed**:
1. CloudWatch logs showed `NoSuchKey` errors during zip generation
2. Added diagnostic S3 HeadObject and ListObjects calls to `handleAddToProject` response body
3. Discovered originals missing from both `images/` and `approved/` paths while thumbnails existed
4. Traced the deletion to the thumbnail generator's idempotency check triggered by S3 CopyObject events

### Bug 2: Async Move Race Condition

**Commit**: `9a2ff77`
**Severity**: Critical - async move Lambda overwrote project data

**Root Cause**: When `addToProject` moves files to `projects/...` and updates DynamoDB (Status=project, ProjectID=xxx), the async move Lambda (triggered earlier during approval) could still be running. The async move would then:
1. Copy files from `projects/...` back to `approved/...`
2. Delete files from `projects/...`
3. Overwrite DynamoDB: Status=approved, clearing ProjectID

**Symptoms**:
- Images briefly appeared in project, then disappeared
- Zip generation found `NoSuchKey` because files were moved back to approved/

**Fix**: Two protections in `handleAsyncMoveFiles`:
1. Early status check: if `img.Status == "project"`, skip the move entirely
2. ConditionExpression on DynamoDB update: `#status <> :projectStatus` prevents overwriting project data
3. ConditionalCheckFailed handler: if the condition fails, just set MoveStatus=complete without overwriting paths

### Bug 3: Frontend API Response Format Mismatch

**Commit**: Previous session
**Severity**: Medium - `getApprovedImages()` failed when paginated response was returned

**Root Cause**: `web/src/services/api.ts` `getApprovedImages()` expected a flat array response but the API returns `{ images: [...], hasMore, nextCursor }`.

**Fix**: Updated `getApprovedImages()` to handle paginated response format with cursor-based pagination.

---

## Debugging Playbook

### When `movedCount` is 0

1. **Verify images exist in DynamoDB**: Call `GET /api/images?state=approved&group={N}&limit=500` and confirm images are returned
2. **Check S3 file existence**: The most reliable method is to add temporary HeadObject/ListObjects calls to the handler and return results in the response body. CloudWatch logs are unreliable under high traffic (500-entry limit returns oldest first).
3. **Check thumbnail generator**: Look for recent processing events in `GET /api/logs?function=ImageThumbnailGenerator&hours=1&filter=all`. If you see `Skipping already processed file` entries for approved/ paths, the thumbnail generator is correctly skipping them. If you see `deleteOriginalFile` entries for approved/ paths, the bug has regressed.

### When zip generation produces empty/small files

1. **Check project images**: `GET /api/projects/{id}/images` - are there actually images assigned?
2. **Check zip logs**: `GET /api/logs?function=ProjectZipGenerator&hours=1&filter=all` - look for `NoSuchKey` errors indicating S3 files are missing
3. **Verify S3 files exist**: For each image in the project, check that `OriginalFile` path actually exists in S3

### When CloudWatch logs aren't showing expected entries

The CloudWatch log API (`GET /api/logs`) returns a maximum of 500 entries from the start of the time window (oldest first). Under heavy traffic, recent entries may be cut off. Workarounds:
- Use `filter=error` to reduce noise (but matches "500" in REPORT durations too)
- Embed diagnostics directly in HTTP response bodies instead of relying on CloudWatch
- Check that the function name matches exactly: `ImageThumbnailGenerator`, `ImageReviewApi`, `ProjectZipGenerator`

### Diagnosing S3 file location issues

Add temporary diagnostic code to the relevant handler:
```go
// HeadObject to check if file exists at expected path
_, headErr := s3Client.HeadObject(&s3.HeadObjectInput{
    Bucket: aws.String(bucketName),
    Key:    aws.String(img.OriginalFile),
})

// ListObjects to find files matching a GUID in a specific directory
dirResult, _ := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
    Bucket:  aws.String(bucketName),
    Prefix:  aws.String("approved/red/2026/01/30/"),
    MaxKeys: aws.Int64(100),
})
```

Return results in the response body for immediate visibility. Remove after diagnosing.

---

## Recovery Procedures

### Recovering images with missing originals

Images approved before the thumbnail generator fix (commit `78921fb`) had their original `.jpg` files deleted. These images still have thumbnails (`.50.jpg`, `.400.jpg`) and DynamoDB records, but the full-resolution original is permanently lost from S3.

**Identification**: Query for approved images and check each with S3 HeadObject. If the original returns 404 but thumbnails exist, the image was affected.

**Options**:
- Re-upload the original images (if available from the source)
- Use the `scripts/recover_group_to_project.go` recovery script if applicable
- Accept the loss and work only with newly uploaded images going forward

### Cleaning up test artifacts

Test runs create temporary projects. To clean up:
```bash
# Via API - list and delete test projects
# Projects named "Zip Test {timestamp}" or "E2E Test Project" are test artifacts
```
