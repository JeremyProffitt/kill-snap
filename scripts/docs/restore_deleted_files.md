# restore_deleted_files.go

Restores deleted files from S3 versioning by removing delete markers.

## Purpose

When S3 versioning is enabled, deleted files aren't actually removed - a "delete marker" is placed that hides the file. This script finds and removes delete markers to restore the original files.

Use this when:
- Files were accidentally deleted from S3
- Need to recover from a mistaken bulk deletion
- Restoring project data after sync issues

## Usage

```bash
# Dry run - show what would be restored
go run restore_deleted_files.go

# Apply the restorations
go run restore_deleted_files.go -apply

# Apply with detailed output
go run restore_deleted_files.go -apply -verbose
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the restorations (default is dry-run) |
| `-verbose` | `false` | Show detailed output for each file |

## How It Works

1. **Lists object versions** in `projects/` prefix
2. **Identifies delete markers** that are the latest version
3. **Classifies files** by type (original, thumbnail, RAW)
4. **Removes delete markers** to restore files (when `-apply` is used)

### S3 Versioning Behavior

```
Before restoration:
  photo.jpg (Delete Marker) <- Latest
  photo.jpg (version 1)     <- Hidden

After restoration:
  photo.jpg (version 1)     <- Latest (visible again)
```

## File Classification

| Type | Pattern |
|------|---------|
| Thumbnail | Contains `.50.` or `.400.` |
| RAW | Extensions: `.raf`, `.cr2`, `.cr3`, `.nef`, `.arw`, `.dng`, `.orf`, `.rw2`, `.pef`, `.srw` |
| Original | `.jpg`, `.jpeg`, `.png` |
| Other | Everything else |

## Example Output

```
Bucket: kill-snap
Region: us-east-2
Mode: DRY-RUN

Scanning for delete markers in projects/...

Found 156 delete markers
  Originals:  52
  Thumbnails: 96
  RAW files:  8

By project:
  grand_canyon_moon: 45 files
  heavy_metal_polaroids: 62 files
  family: 49 files

This was a DRY RUN. To restore these files, run:
  go run restore_deleted_files.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `AWS_REGION` | `us-east-2` | AWS region |

## Prerequisites

- **S3 versioning must be enabled** on the bucket
- Delete markers must exist (files deleted after versioning was enabled)
- Appropriate AWS permissions to list versions and delete objects

## What Gets Restored

- All files in `projects/` prefix with delete markers
- Includes originals, thumbnails, and RAW files
- Does not restore files from `incoming/`, `images/`, or other prefixes

## When to Use

- After accidental file deletions
- When sync function incorrectly removed files
- Recovering from bulk operations gone wrong
- Restoring project data after DynamoDB/S3 sync issues

## Risk Level: Low

- Only removes delete markers (restores hidden files)
- Does not create new data
- Does not modify file contents
- Idempotent - running again has no effect if files already restored

## Limitations

1. Only scans `projects/` prefix
2. Cannot restore files deleted before versioning was enabled
3. Cannot restore files if all versions have been permanently deleted
4. Lifecycle policies may have already removed old versions

## Checking Versioning Status

Verify S3 versioning is enabled:
```bash
aws s3api get-bucket-versioning --bucket kill-snap
```

Expected output:
```json
{
    "Status": "Enabled"
}
```

## Related Scripts

- [recover_project_images.go](recover_project_images.md) - Rebuild DynamoDB records after restoration
- [fix_thumbnail_paths.go](fix_thumbnail_paths.md) - Fix thumbnail paths after restoration
- [regenerate_thumbnails.go](regenerate_thumbnails.md) - Regenerate if thumbnails can't be restored
