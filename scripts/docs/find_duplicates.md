# find_duplicates.go

Finds and removes duplicate DynamoDB records that have the same `OriginalFile` path.

## Purpose

Duplicate records can occur when:
- Recovery scripts create new records without checking for existing ones with different GUIDs
- Images are processed multiple times through different code paths
- Manual data corrections create overlapping records

Duplicates cause issues like images appearing in multiple views simultaneously (e.g., both "ungrouped" and "Group 1").

## Usage

```bash
# Dry run - find duplicates and show what would be deleted
go run find_duplicates.go

# Apply deletions
go run find_duplicates.go -apply
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply deletions (default is dry-run) |

## How It Works

1. **Scans all records** in ImageMetadata table
2. **Groups by OriginalFile** path
3. **Identifies duplicates** - files with more than one record
4. **Ranks records** to determine which to keep:
   - Prefer records with `GroupNumber > 0`
   - Prefer records with `Status = "project"`
   - Prefer records with `ProjectID` set
   - Fall back to `UpdatedDateTime` (most recent)
5. **Deletes inferior duplicates** while keeping the best record

## Record Selection Logic

```go
// Priority order (highest to lowest):
1. Has GroupNumber > 0 (user has assigned to a group)
2. Has Status = "project" (part of a project)
3. Has ProjectID set (linked to a project)
4. Most recent UpdatedDateTime
```

## Example Output

```
Image Table: kill-snap-ImageMetadata
Mode: DRY-RUN

Scanning all records...
Total records: 4966
Unique files: 4242

Duplicate: projects/family/2025/12/10/001768020015.jpg (3 records)
  KEEP: ff100508-a137-4eb2-bb0d-c19d4aaaa6fd (Status=project, Group=5, ProjectID=3ec7afa0-325f-4864-8609-813a695a9e82)
  DELETE: 001768020015 (Status=project, Group=0)
  DELETE: dbc42e07-efb3-44a5-8a24-52e7c4a16b3c (Status=inbox, Group=0)

Duplicate: projects/heavy_metal_polaroids/2025/12/08/scan010.jpg (3 records)
  KEEP: 453bdd2b-5253-4a87-867d-7b295f625771 (Status=project, Group=1, ProjectID=0517314f-bed5-4b38-bfa7-0a0742918456)
  DELETE: scan010 (Status=project, Group=0)
  DELETE: b13a25bd-ba29-4341-9a4f-423831c21db3 (Status=inbox, Group=0)

=== Summary ===
Files with duplicates: 626
Records to delete: 724

This was a DRY RUN. To delete duplicates, run:
  go run find_duplicates.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB table name |

## Common Duplicate Patterns

### Pattern 1: Recovery Script Duplicates
```
KEEP: uuid-based-guid (Status=project, Group=1, ProjectID=xxx)
DELETE: filename-based-guid (Status=project, Group=0)
DELETE: another-uuid (Status=inbox, Group=0)
```

### Pattern 2: Inbox + Project Duplicates
```
KEEP: project-record (Status=project, GroupNumber=3)
DELETE: inbox-record (Status=inbox)
```

## When to Use

- Images appearing in multiple views simultaneously
- After running recovery scripts
- Record count seems higher than expected
- UI shows inconsistent data for the same image

## Risk Level: High

- **Permanently deletes DynamoDB records**
- Always run dry-run first
- Verify the "KEEP" selections are correct
- Cannot be undone (except from DynamoDB PITR backup)

## Safety Features

- Dry-run by default
- Shows detailed output of what will be kept/deleted
- Selection logic prefers records with user data (GroupNumber, ProjectID)

## Recovery Options

If wrong records are deleted:
1. **DynamoDB Point-in-Time Recovery** - Restore table to before deletion
2. **[recover_project_images.go](recover_project_images.md)** - Rebuild records from S3

## Related Scripts

- [recover_project_images.go](recover_project_images.md) - May create duplicates if run incorrectly
- [backfill_status.go](backfill_status.md) - Adds Status field to records
