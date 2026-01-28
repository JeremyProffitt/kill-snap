# fix_thumbnail_paths.go

Fixes NULL or empty thumbnail paths in DynamoDB records by deriving the correct paths from the OriginalFile path.

## Purpose

Project images may have `Thumbnail50` and `Thumbnail400` fields set to NULL or empty if:
- Records were created by recovery scripts without thumbnail info
- Thumbnails exist in S3 but paths weren't recorded in DynamoDB
- Data migration issues

This script derives thumbnail paths from the original file path and updates records when the thumbnails exist in S3.

## Usage

```bash
# Dry run - show what would be fixed
go run fix_thumbnail_paths.go

# Apply fixes
go run fix_thumbnail_paths.go -apply

# Apply with detailed output
go run fix_thumbnail_paths.go -apply -verbose
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the fixes (default is dry-run) |
| `-verbose` | `false` | Show detailed output for each record |

## How It Works

1. **Scans for project images** with missing thumbnail paths
   - Filter: `Status = "project"` AND `OriginalFile begins_with "projects/"`
   - Checks if `Thumbnail50` or `Thumbnail400` is empty

2. **Derives thumbnail paths** from original file:
   ```
   OriginalFile: projects/south_west/2024/02/09/Roll8633-8652.jpg
   Thumbnail50:  projects/south_west/2024/02/09/Roll8633-8652.50.jpg
   Thumbnail400: projects/south_west/2024/02/09/Roll8633-8652.400.jpg
   ```

3. **Verifies thumbnails exist** in S3 before updating

4. **Updates DynamoDB** with correct paths (also checks for RAW files)

## Path Derivation Logic

```
Original:   {dir}/{basename}.{ext}
Thumb 50:   {dir}/{basename}.50.{ext}
Thumb 400:  {dir}/{basename}.400.{ext}
```

Example:
```
Original:   projects/miami/2025/12/08/001522830011.jpg
Thumb 50:   projects/miami/2025/12/08/001522830011.50.jpg
Thumb 400:  projects/miami/2025/12/08/001522830011.400.jpg
```

## Example Output

```
Image Table: kill-snap-ImageMetadata
S3 Bucket: kill-snap
Mode: DRY-RUN

Scanning for project images with missing thumbnail paths...
Found 156 records needing thumbnail path fixes

  Roll8633-8652:
    Thumb50:  projects/south_west/2024/02/09/Roll8633-8652.50.jpg (exists: true)
    Thumb400: projects/south_west/2024/02/09/Roll8633-8652.400.jpg (exists: true)
  scan107:
    Thumb50:  projects/heavy_metal_polaroids/2025/12/08/scan107.50.jpg (exists: true)
    Thumb400: projects/heavy_metal_polaroids/2025/12/08/scan107.400.jpg (exists: true)

=== Summary ===
Records needing fixes: 156
Would fix:             156

This was a DRY RUN. To apply fixes, run:
  go run fix_thumbnail_paths.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB table |

## What Gets Updated

- `Thumbnail50` - 50px thumbnail path
- `Thumbnail400` - 400px thumbnail path
- `RawFile` - RAW file path (if found and not already set)

Only updates fields when:
1. Current value is empty/NULL
2. Derived file exists in S3

## RAW File Detection

Also searches for associated RAW files:
- Extensions checked: `.raf`, `.RAF`, `.cr2`, `.CR2`, `.nef`, `.NEF`, `.arw`, `.ARW`, `.dng`, `.DNG`
- Updates `RawFile` field if found

## When to Use

- After running [recover_project_images.go](recover_project_images.md)
- When thumbnails exist but paths are NULL in DynamoDB
- Images showing broken thumbnail icons in UI

## Risk Level: Low

- Only updates empty/NULL fields
- Does not overwrite existing values
- Verifies S3 file existence before updating

## Troubleshooting

### "exists: false" for all thumbnails

Thumbnails may not exist in S3. Use:
- [restore_deleted_files.go](restore_deleted_files.md) - Restore from versioning
- [regenerate_thumbnails.go](regenerate_thumbnails.md) - Generate new thumbnails

### Windows path issues

The script uses `path` (not `filepath`) to ensure forward slashes for S3 keys.

## Related Scripts

- [regenerate_thumbnails.go](regenerate_thumbnails.md) - Generate missing thumbnails
- [restore_deleted_files.go](restore_deleted_files.md) - Restore deleted thumbnails
- [recover_project_images.go](recover_project_images.md) - Creates records that may need this fix
