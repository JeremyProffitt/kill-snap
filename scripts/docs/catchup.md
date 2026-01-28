# catchup.go

Multi-purpose script for managing image processing pipeline issues.

## Purpose

Handles various image processing scenarios:
- Finding and reprocessing unprocessed images
- Managing SQS queues (main queue and DLQ)
- Watching processing progress in real-time
- Fixing orphaned images
- Migrating old file naming conventions

## Usage

```bash
# List unprocessed files and show stats
go run catchup.go

# Push unprocessed files to SQS (default limit: 100)
go run catchup.go -push

# Push first 500 unprocessed files
go run catchup.go -push -limit 500

# Push ALL unprocessed files (no limit)
go run catchup.go -push -nolimit

# Show what would be pushed without actually pushing
go run catchup.go -dry-run

# Watch SQS queue and CloudWatch logs for errors
go run catchup.go -watch

# Move DLQ messages back to main queue for retry
go run catchup.go -redrive

# Fix orphaned images (cross-reference S3 and DynamoDB)
go run catchup.go -orphans

# Migrate old file naming to new GUID format
go run catchup.go -migrate
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-push` | `false` | Push unprocessed files to SQS |
| `-limit` | `100` | Maximum number of files to push |
| `-nolimit` | `false` | Push all files (no limit) |
| `-dry-run` | `false` | Preview without making changes |
| `-watch` | `false` | Watch queue and logs in real-time |
| `-redrive` | `false` | Move DLQ messages back to main queue |
| `-orphans` | `false` | Fix orphaned images |
| `-migrate` | `false` | Migrate old naming convention |

## Modes of Operation

### Default Mode (Stats)
Lists unprocessed files in `incoming/` folder and shows statistics.

### Push Mode
Sends S3 event notifications to SQS to trigger thumbnail processing:
- Scans `incoming/` for JPG/JPEG files
- Checks DynamoDB for existing records
- Sends S3 event message to SQS for processing

### Watch Mode
Real-time monitoring:
- Polls SQS queue depth every 30 seconds
- Streams CloudWatch logs for errors
- Runs for up to 2 hours

### Redrive Mode
Moves failed messages from DLQ back to main queue for retry:
- Receives messages from DLQ
- Sends to main queue
- Deletes from DLQ

### Orphans Mode
Cross-references S3 and DynamoDB to find:
- S3 files without DynamoDB records
- DynamoDB records without S3 files

### Migrate Mode
Converts old file naming (original filename) to new GUID-based naming.

## Example Output

```
=== Kill-Snap Image Catchup Tool ===
Bucket: kill-snap
Table: kill-snap-ImageMetadata
Region: us-east-2

Scanning incoming/ folder...
Found 156 JPG/JPEG files in incoming/

Checking DynamoDB for existing records...
  - 142 already processed
  - 14 need processing

Files needing processing:
  1. incoming/2026/01/28/IMG_0001.JPG
  2. incoming/2026/01/28/IMG_0002.JPG
  ...
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BUCKET_NAME` | `kill-snap` | S3 bucket name |
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB table |
| `SQS_QUEUE_URL` | (hardcoded) | Main SQS queue URL |
| `AWS_REGION` | `us-east-2` | AWS region |

## AWS Resources Used

- **S3**: Scans `incoming/` prefix
- **DynamoDB**: Queries ImageMetadata table
- **SQS**: Main processing queue and DLQ
- **CloudWatch Logs**: Lambda function logs

## When to Use

- Images uploaded but not appearing in UI
- Processing pipeline stuck or backed up
- After Lambda errors causing failed processing
- To monitor bulk upload progress
- To retry failed image processing

## Risk Level: Medium

- Push mode creates SQS messages that trigger Lambda
- Redrive mode moves messages between queues
- Use `-dry-run` first to preview actions

## Related Scripts

- [regenerate_thumbnails.go](regenerate_thumbnails.md) - Direct thumbnail generation
- [recover_project_images.go](recover_project_images.md) - Rebuild DynamoDB records
