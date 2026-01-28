# Kill-Snap Maintenance Scripts

This directory contains Go scripts for maintaining, recovering, and troubleshooting the kill-snap image management system.

## Quick Reference

| Script | Purpose | Risk Level |
|--------|---------|------------|
| [backfill_status.go](docs/backfill_status.md) | Add Status field to records missing it | Low |
| [catchup.go](docs/catchup.md) | Reprocess unprocessed images, manage SQS queues | Medium |
| [cleanup_test_folders.go](docs/cleanup_test_folders.md) | Remove orphan test folders from S3 | Medium |
| [extract-raf-preview.go](docs/extract-raf-preview.md) | Extract JPEG previews from RAF files | Low |
| [find_duplicates.go](docs/find_duplicates.md) | Find and remove duplicate DynamoDB records | High |
| [fix_thumbnail_paths.go](docs/fix_thumbnail_paths.md) | Fix NULL thumbnail paths in DynamoDB | Low |
| [recover_project_images.go](docs/recover_project_images.md) | Rebuild DynamoDB records from S3 files | Medium |
| [regenerate_thumbnails.go](docs/regenerate_thumbnails.md) | Regenerate missing thumbnails | Low |
| [restore_deleted_files.go](docs/restore_deleted_files.md) | Restore S3 files from versioning | Low |
| [update_project_counts.go](docs/update_project_counts.md) | Sync project ImageCount values | Low |

## Prerequisites

All scripts require:
- Go 1.21 or later
- AWS credentials configured (`~/.aws/credentials` or environment variables)
- Access to the `us-east-2` region

Install dependencies:
```bash
cd scripts
go mod download
```

## Safety Features

**All scripts support dry-run mode by default.** Run without `-apply` to preview changes:

```bash
# Preview what would happen
go run script_name.go

# Actually make changes
go run script_name.go -apply
```

## Common Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AWS_REGION` | `us-east-2` | AWS region |
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB images table |
| `PROJECT_TABLE` | `kill-snap-Projects` | DynamoDB projects table |

## Script Categories

### Data Recovery

Scripts for recovering from data loss:

1. **[restore_deleted_files.go](docs/restore_deleted_files.md)** - Restore S3 files from versioning
2. **[recover_project_images.go](docs/recover_project_images.md)** - Rebuild DynamoDB records
3. **[fix_thumbnail_paths.go](docs/fix_thumbnail_paths.md)** - Fix broken thumbnail references
4. **[regenerate_thumbnails.go](docs/regenerate_thumbnails.md)** - Regenerate missing thumbnails

### Data Maintenance

Scripts for routine maintenance:

1. **[find_duplicates.go](docs/find_duplicates.md)** - Remove duplicate records
2. **[update_project_counts.go](docs/update_project_counts.md)** - Fix project image counts
3. **[cleanup_test_folders.go](docs/cleanup_test_folders.md)** - Clean up test data
4. **[backfill_status.go](docs/backfill_status.md)** - Backfill Status field

### Image Processing

Scripts for image processing tasks:

1. **[catchup.go](docs/catchup.md)** - Reprocess failed/stuck images
2. **[extract-raf-preview.go](docs/extract-raf-preview.md)** - Extract JPEGs from RAW files

## Recommended Recovery Workflow

When recovering from data issues, run scripts in this order:

```bash
# 1. Restore deleted S3 files from versioning
go run restore_deleted_files.go -apply

# 2. Rebuild missing DynamoDB records
go run recover_project_images.go -apply

# 3. Fix thumbnail paths that are NULL
go run fix_thumbnail_paths.go -apply

# 4. Regenerate any still-missing thumbnails
go run regenerate_thumbnails.go -apply

# 5. Remove any duplicate records created
go run find_duplicates.go -apply

# 6. Update project image counts
go run update_project_counts.go -apply
```

## Troubleshooting

### Script fails with AWS credentials error

Ensure AWS credentials are configured:
```bash
aws sts get-caller-identity
```

### Script times out

Some scripts scan large tables. Use environment variables to target specific data:
```bash
IMAGE_TABLE=kill-snap-ImageMetadata go run script.go
```

### Go module errors

Update dependencies:
```bash
cd scripts
go mod tidy
go mod download
```

## Contributing

When adding new scripts:

1. Follow the existing pattern with dry-run mode
2. Add comprehensive logging
3. Create documentation in `docs/script_name.md`
4. Update this README with the new script
5. Test in dry-run mode first

## Related Documentation

- [fix-plan.md](../fix-plan.md) - Automatic remediation plan
- [plan.md](../plan.md) - Original recovery plan
- [CLAUDE.md](../CLAUDE.md) - Project conventions
