# cleanup_test_folders.go

Removes orphan test folders from S3 that don't have corresponding project entries in DynamoDB.

## Purpose

During development and testing, project folders may be created in S3 without proper DynamoDB project records. This script identifies and removes these orphan folders to clean up storage.

## Usage

```bash
# Dry run - show what would be deleted
go run cleanup_test_folders.go

# Apply deletions
go run cleanup_test_folders.go -apply
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the deletions (default is dry-run) |

## How It Works

1. **Fetches valid project prefixes** from DynamoDB Projects table
2. **Lists all folders** in `s3://kill-snap/projects/`
3. **Identifies orphans** - folders not in the projects table
4. **Counts files** in each orphan folder
5. **Deletes all objects** in orphan folders (when `-apply` is used)

## Example Output

```
S3 Bucket: kill-snap
Project Table: kill-snap-Projects
Mode: DRY-RUN

Fetching valid project prefixes...
Found 24 valid project prefixes

Listing S3 project folders...
Found 34 S3 folders

Found 10 orphan folders:
  a48fd74b-5d96-4c1d-8be1-cb452fbe1ef4: 32 files
  dadfa: 9 files
  dd: 6 files
  ddadf: 25 files
  equip: 46 files
  test: 30 files
  test2: 14 files
  test22: 10 files
  yadatofu: 1 files
  yayaya: 5 files

Total files to delete: 178

This was a DRY RUN. To delete these folders, run:
  go run cleanup_test_folders.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `PROJECT_TABLE` | `kill-snap-Projects` | DynamoDB projects table |

## Safety Features

- **Dry-run by default** - Always preview before deleting
- **Only deletes from projects/** - Won't touch other S3 prefixes
- **Cross-references DynamoDB** - Only deletes folders without project records

## Typical Orphan Folder Names

Common patterns for test folders:
- Random strings: `dadfa`, `ddadf`, `yayaya`
- Test prefixes: `test`, `test2`, `test22`
- UUIDs without project records
- Temporary names: `equip`, `yadatofu`

## When to Use

- Periodic cleanup of test data
- After development/testing sessions
- To reduce S3 storage costs
- Before auditing storage usage

## Risk Level: Medium

- **Permanently deletes S3 objects**
- S3 versioning may allow recovery if enabled
- Always run dry-run first to verify folders

## What Gets Deleted

- All objects under `projects/{orphan_folder}/`
- Includes originals, thumbnails, RAW files

## What Is NOT Deleted

- Folders that have matching project records in DynamoDB
- Any other S3 prefixes (images/, incoming/, deleted/, etc.)

## Related Scripts

- [restore_deleted_files.go](restore_deleted_files.md) - Can restore files if versioning is enabled
