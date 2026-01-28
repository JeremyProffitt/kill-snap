# backfill_status.go

Backfills the `Status` field for existing DynamoDB records that don't have it set.

## Purpose

When the `Status` field was added to the schema, existing records didn't have this field populated. This script analyzes each record and sets the appropriate status based on other fields.

## Usage

```bash
# Dry run - preview what would be updated
go run backfill_status.go

# Apply updates
go run backfill_status.go -apply

# Apply with detailed output
go run backfill_status.go -apply -verbose
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the updates (default is dry-run) |
| `-verbose` | `false` | Show detailed output for each record |

## Status Determination Logic

The script determines status based on these rules:

1. **`project`** - Record has `ProjectID` set
2. **`inbox`** - Record has `Reviewed=false` or empty
3. **`approved`** - Record has `Reviewed=true` AND `GroupNumber > 0`
4. **`rejected`** - Record has `Reviewed=true` AND `GroupNumber = 0`

## Example Output

```
Table: kill-snap-ImageMetadata
Region: us-east-2
Mode: DRY-RUN

Scanning all records...
  Processing page 1 (1000 records)...
  Processing page 2 (1000 records)...
  Processing page 3 (966 records)...

=== Summary ===
Total records scanned:      2966
Already has Status:         2217
Needs Status update:        749

Status breakdown:
  -> inbox:     442
  -> approved:  156
  -> rejected:  89
  -> project:   62

This was a DRY RUN. To apply these updates, run:
  go run backfill_status.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB table name |
| `AWS_REGION` | `us-east-2` | AWS region |

## DynamoDB Operations

- **Read**: Scans entire table
- **Write**: Updates `Status` and `UpdatedDateTime` fields

## When to Use

- After schema changes that add the Status field
- When records are missing the Status field (causing them to not appear in UI)
- As part of data migration

## Risk Level: Low

- Only adds missing Status field
- Does not delete or modify existing Status values
- Idempotent - safe to run multiple times

## Related Scripts

- [recover_project_images.go](recover_project_images.md) - Creates records with Status already set
