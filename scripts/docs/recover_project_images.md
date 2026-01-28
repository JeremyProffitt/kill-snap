# recover_project_images.go

Rebuilds DynamoDB records for project images by scanning S3 and creating missing records.

## Purpose

DynamoDB records may be missing for project images when:
- Files were uploaded directly to S3 bypassing the normal pipeline
- Records were accidentally deleted
- Data migration issues
- Previous recovery attempts failed partially

This script scans S3 project folders, finds original images, and creates DynamoDB records for any that are missing.

## Usage

```bash
# Dry run - show what would be created
go run recover_project_images.go

# Apply the changes
go run recover_project_images.go -apply

# Apply with detailed output
go run recover_project_images.go -apply -verbose
```

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-apply` | `false` | Apply the changes (default is dry-run) |
| `-verbose` | `false` | Show detailed output for each record |

## How It Works

1. **Loads projects** from DynamoDB Projects table
   - Creates S3Prefix -> ProjectID mapping

2. **Scans S3** for original images in `projects/` prefix
   - Identifies original images (excludes thumbnails, RAW files, zips)
   - Finds related files (thumbnails, RAW files)

3. **Checks for existing records** in DynamoDB
   - Checks by ImageGUID (filename-based or UUID)
   - Also checks by OriginalFile path (prevents duplicates with different GUIDs)

4. **Creates missing records** with:
   - `Status = "project"`
   - `Reviewed = "true"`
   - Thumbnail paths (if thumbnails exist)
   - RAW file path (if RAW exists)
   - Links to correct ProjectID

5. **Updates project ImageCount** values

## Record Fields

Created records include:

| Field | Value |
|-------|-------|
| `ImageGUID` | UUID from filename or filename as GUID |
| `OriginalFile` | S3 key of the original image |
| `OriginalFilename` | Base filename without extension |
| `RawFile` | Path to RAW file if found |
| `Bucket` | S3 bucket name |
| `Thumbnail50` | Path to 50px thumbnail if exists |
| `Thumbnail400` | Path to 400px thumbnail if exists |
| `Status` | "project" |
| `ProjectID` | Project UUID from mapping |
| `Reviewed` | "true" |
| `FileSize` | Size from S3 |
| `InsertedDateTime` | Current timestamp |
| `UpdatedDateTime` | Current timestamp |

## Example Output

```
S3 Bucket: kill-snap (region: us-east-2)
Image Table: kill-snap-ImageMetadata (region: us-east-2)
Project Table: kill-snap-Projects (region: us-east-2)
Mode: DRY-RUN

Loading projects...
Loaded 24 projects

Scanning S3 for project images...
Found 4242 original images in project folders

Checking for existing DynamoDB records...

Images with existing records: 4100
Images needing records:       142

By project:
  grand_canyon_moon: 38 images
  heavy_metal_polaroids: 52 images
  family: 52 images

This was a DRY RUN. To create these records, run:
  go run recover_project_images.go -apply
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `S3_BUCKET` | `kill-snap` | S3 bucket name |
| `IMAGE_TABLE` | `kill-snap-ImageMetadata` | DynamoDB images table |
| `PROJECT_TABLE` | `kill-snap-Projects` | DynamoDB projects table |

## Duplicate Prevention

The script prevents duplicate records by checking:
1. **By ImageGUID** - Direct lookup using the partition key
2. **By OriginalFile path** - Scan for any record with matching path

This ensures images already in DynamoDB (possibly with different GUIDs) are not duplicated.

## Supported File Types

### Original Images
- `.jpg`, `.jpeg`, `.png`

### RAW Files (associated with originals)
- `.raf`, `.RAF` (Fujifilm)
- `.cr2`, `.CR2`, `.cr3`, `.CR3` (Canon)
- `.nef`, `.NEF` (Nikon)
- `.arw`, `.ARW` (Sony)
- `.dng`, `.DNG` (Adobe)
- `.orf`, `.rw2`, `.pef`, `.srw`

### Excluded
- Thumbnail files (`.50.`, `.400.`)
- ZIP archives

## When to Use

- Images visible in S3 but not appearing in UI
- After S3 data restoration
- When DynamoDB records were accidentally deleted
- After direct S3 uploads
- To rebuild project data

## Risk Level: Medium

- Creates new DynamoDB records
- Sets records as `Reviewed = "true"` and `Status = "project"`
- Always run dry-run first to verify scope
- Duplicate prevention is in place but verify project mapping

## Recovery After Incorrect Run

If records are created incorrectly:
1. Use [find_duplicates.go](find_duplicates.md) to identify and remove duplicates
2. Manually delete incorrect records if needed
3. DynamoDB PITR can restore to before the script ran

## Related Scripts

- [find_duplicates.go](find_duplicates.md) - Find and remove duplicate records
- [fix_thumbnail_paths.go](fix_thumbnail_paths.md) - Fix NULL thumbnail paths
- [update_project_counts.go](update_project_counts.md) - Sync project image counts
