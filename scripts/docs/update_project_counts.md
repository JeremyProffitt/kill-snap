# update_project_counts.go

Updates project `ImageCount` values to match actual DynamoDB record counts.

## Purpose

Project `ImageCount` values can become out of sync when:
- Images are added or removed outside the normal workflow
- Recovery scripts create/delete records
- Manual database modifications
- Bugs in image processing pipeline

This script counts actual images per project and updates any incorrect `ImageCount` values.

## Usage

```bash
# Dry run - show what would be updated
go run update_project_counts.go

# Apply the updates
go run update_project_counts.go -apply
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the updates (default is dry-run) |

## How It Works

1. **Fetches all projects** from Projects table
2. **Counts images per project** by scanning ImageMetadata table
   - Filter: `Status = "project"`
   - Groups by `ProjectID`
3. **Compares counts** with stored `ImageCount` values
4. **Updates incorrect values** (when `-apply` is used)

## Example Output

```
Image Table: kill-snap-ImageMetadata
Project Table: kill-snap-Projects
Mode: DRY-RUN

Fetching projects...
Found 24 projects

Counting images per project...
  grand_canyon_moon (abc123-def456): 45 -> 52
  heavy_metal_polaroids (xyz789-abc012): 100 -> 98
  family (def456-ghi789): 200 -> 203

=== Summary ===
Total projects: 24
Would update: 3

This was a DRY RUN. To apply updates, run:
  go run update_project_counts.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB images table |
| `PROJECT_TABLE` | `kill-snap-Projects` | DynamoDB projects table |

## What Gets Updated

- `ImageCount` field in the Projects table
- Only updates if current value differs from actual count
- Does not modify any other fields

## Count Logic

Images are counted when:
- `Status = "project"`
- `ProjectID` matches the project's `ProjectID`

Images NOT counted:
- `Status = "inbox"` (not assigned to project)
- `Status = "deleted"` (soft deleted)
- Records without `ProjectID`

## When to Use

- After running recovery scripts
- When UI shows incorrect image counts
- As part of routine maintenance
- After bulk operations on images
- Before auditing project data

## Risk Level: Very Low

- Only updates `ImageCount` field
- Does not modify images or other data
- Counts are derived from actual records
- Easily reversible (just run again)

## Scheduling

Consider running periodically:
- After batch processing operations
- Weekly as part of maintenance
- After any recovery script runs

## Performance Notes

- Performs full table scan of ImageMetadata
- May be slow for large tables (thousands of records)
- Efficient for typical project sizes

## Related Scripts

- [recover_project_images.go](recover_project_images.md) - Creates records that affect counts
- [find_duplicates.go](find_duplicates.md) - Removes duplicates that affect counts
