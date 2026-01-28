# regenerate_thumbnails.go

Regenerates missing thumbnails for project images by downloading originals and creating new thumbnails.

## Purpose

Thumbnails may be missing when:
- Original Lambda thumbnail generation failed
- Thumbnails were accidentally deleted
- Images were recovered without their thumbnails
- Corrupted uploads

This script downloads original images, generates 50px and 400px thumbnails, uploads them to S3, and updates DynamoDB records.

## Usage

```bash
# Dry run - show what would be regenerated
go run regenerate_thumbnails.go

# Apply the regeneration
go run regenerate_thumbnails.go -apply
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the regeneration (default is dry-run) |

## How It Works

1. **Scans DynamoDB** for project images
   - Filter: `Status = "project"` AND `OriginalFile begins_with "projects/"`

2. **Identifies missing thumbnails**
   - Checks if `Thumbnail50` or `Thumbnail400` fields are empty
   - Verifies thumbnails don't exist in S3

3. **Regenerates thumbnails** (when `-apply` is used)
   - Downloads original image from S3
   - Decodes JPEG using imaging library
   - Creates 50px thumbnail (fit within 50x50, Lanczos filter, 80% quality)
   - Creates 400px thumbnail (fit within 400x400, Lanczos filter, 85% quality)
   - Uploads thumbnails to S3
   - Updates DynamoDB record with thumbnail paths

## Thumbnail Specifications

| Size | Dimensions | Quality | Filter |
|------|-----------|---------|--------|
| 50px | Fit 50x50 | 80% | Lanczos |
| 400px | Fit 400x400 | 85% | Lanczos |

**Note**: "Fit" means the image is scaled to fit within the specified dimensions while maintaining aspect ratio.

## Path Derivation

```
Original:   projects/miami/2025/12/08/photo.jpg
Thumb 50:   projects/miami/2025/12/08/photo.50.jpg
Thumb 400:  projects/miami/2025/12/08/photo.400.jpg
```

## Example Output

```
Image Table: kill-snap-ImageMetadata
S3 Bucket: kill-snap
Mode: DRY-RUN

Scanning for project images needing thumbnail regeneration...
Found 38 records needing thumbnail regeneration

  abc123-def456: projects/grand_canyon_moon/2025/01/10/DSCF0001.jpg
    Would generate: projects/grand_canyon_moon/2025/01/10/DSCF0001.50.jpg, projects/grand_canyon_moon/2025/01/10/DSCF0001.400.jpg
  ...

=== Summary ===
Records needing regeneration: 38
Would regenerate:             38

This was a DRY RUN. To regenerate thumbnails, run:
  go run regenerate_thumbnails.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB images table |

## Dependencies

Requires the `imaging` package:
```bash
go get github.com/disintegration/imaging
```

## What Gets Created

- `{basename}.50.jpg` - 50px thumbnail in S3
- `{basename}.400.jpg` - 400px thumbnail in S3
- DynamoDB fields `Thumbnail50` and `Thumbnail400` updated

## Error Handling

Common errors and their causes:

| Error | Cause |
|-------|-------|
| `ERROR downloading` | Original file doesn't exist in S3 |
| `ERROR decoding` | Corrupted or invalid JPEG file |
| `ERROR encoding` | Memory issues or invalid image data |
| `ERROR uploading` | S3 permissions or network issues |

## When to Use

- Images showing broken thumbnail icons in UI
- After running [recover_project_images.go](recover_project_images.md)
- After restoring deleted files
- When Lambda thumbnail generation fails repeatedly

## Risk Level: Low

- Only creates new files in S3
- Updates existing DynamoDB records
- Does not modify or delete original images
- Overwrites existing thumbnails if regenerated

## Limitations

1. Only processes project images (`Status = "project"`)
2. Only handles JPEG images
3. Cannot process RAW files directly (use [extract-raf-preview.go](extract-raf-preview.md) first)
4. Corrupted original images will fail

## Performance Notes

- Downloads entire original image to memory
- Processing time depends on original image size
- Network-bound for large images
- Consider running with limited batches for large numbers

## Related Scripts

- [fix_thumbnail_paths.go](fix_thumbnail_paths.md) - Fix NULL paths when thumbnails exist
- [extract-raf-preview.go](extract-raf-preview.md) - Extract JPEGs from RAW files
- [restore_deleted_files.go](restore_deleted_files.md) - Restore deleted thumbnails from versioning
