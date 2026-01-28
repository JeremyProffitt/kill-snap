# extract-raf-preview.go

Extracts embedded JPEG preview images from Fujifilm RAF (RAW) files.

## Purpose

Fujifilm RAF files contain embedded full-resolution JPEG previews. This script extracts these previews for cases where the Lambda thumbnail generator fails to process RAW files, or when you need JPEGs without running the full processing pipeline.

## Usage

```bash
# Process up to 10 RAF files (default)
go run extract-raf-preview.go

# Process a specific file
go run extract-raf-preview.go -file incoming/2026/01/28/DSCF0001.RAF

# Process more files
go run extract-raf-preview.go -limit 50

# Dry run - show what would be done
go run extract-raf-preview.go -dry-run
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-dry-run` | `false` | Show what would be done without making changes |
| `-limit` | `10` | Maximum number of files to process |
| `-file` | `""` | Process a single file by S3 key |

## How It Works

### RAF File Structure

Fujifilm RAF files have a specific structure:
```
Offset 0:   "FUJIFILMCCD-RAW " (16 bytes header)
Offset 16:  Format version (4 bytes)
Offset 20:  Camera ID (8 bytes)
Offset 28:  RAF version string (4 bytes)
Offset 84:  JPEG image offset (4 bytes, big-endian)
Offset 88:  JPEG image length (4 bytes, big-endian)
```

### Extraction Methods

1. **Header-based extraction** (preferred)
   - Reads JPEG offset and length from RAF header
   - Fast and accurate for valid RAF files

2. **Scan-based extraction** (fallback)
   - Scans file for JPEG markers (FF D8 FF ... FF D9)
   - Finds all embedded JPEGs
   - Returns the largest one (usually the full preview)

### Output

- Creates `{basename}.JPG` in the same S3 location as the RAF file
- Skips if JPG already exists
- Skips files smaller than 20MB (likely corrupted)

## Example Output

```
=== RAF Preview Extractor ===
Bucket: kill-snap
Region: us-east-2

Scanning for RAF files in incoming/...
Found 5 RAF files with content

Processing: incoming/2026/01/28/DSCF0001.RAF
  Downloaded: 52428800 bytes
  Valid Fujifilm RAF file detected
  RAF header: JPEG offset=16896, length=8234567
  Extracted JPEG from header location: 8234567 bytes
  Uploading to bucket=kill-snap key=incoming/2026/01/28/DSCF0001.JPG size=8234567
  Uploaded: incoming/2026/01/28/DSCF0001.JPG (8234567 bytes) ETag: "abc123..."

Summary: Processed 5 files, 0 errors
```

## Supported RAW Formats

Currently optimized for:
- **Fujifilm RAF** - Full support with header parsing

The scan-based fallback may work with other formats:
- Canon CR2/CR3
- Nikon NEF
- Sony ARW
- Adobe DNG

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| (none) | - | Uses hardcoded bucket and region |

**Note**: This script uses hardcoded values:
- Bucket: `kill-snap`
- Region: `us-east-2`

## File Size Requirements

- **Minimum**: 20MB (smaller files are skipped as likely corrupted)
- **Typical RAF size**: 50-70MB for Fujifilm cameras

## When to Use

- Lambda fails to process RAW files
- Need quick JPEG extraction without full pipeline
- Bulk processing of RAW files uploaded directly
- Recovering previews from corrupted processing runs

## Risk Level: Low

- Only creates new JPG files
- Skips existing JPGs (no overwrite)
- Does not modify or delete RAF files

## Known Limitations

1. Only scans `incoming/` prefix
2. Hardcoded bucket and region
3. May extract thumbnail instead of full preview for some camera models

## Related Scripts

- [catchup.go](catchup.md) - Reprocess images through normal pipeline
- [regenerate_thumbnails.go](regenerate_thumbnails.md) - Generate thumbnails from JPEGs
