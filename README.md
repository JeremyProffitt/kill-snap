# Image Thumbnail Generator

Automated S3-triggered image processing service that generates thumbnails and stores metadata in DynamoDB.

## Features

- **S3 Event-Driven**: Automatically triggers when JPG files are uploaded to S3
- **Dual Thumbnails**: Generates 50px and 400px height thumbnails
- **Metadata Storage**: Stores image information in DynamoDB including:
  - GUID identifier
  - Original file location
  - Thumbnail file names
  - EXIF data
  - Related files (same base name, different extensions)
- **Protected Resources**: S3 bucket and DynamoDB table are configured with deletion protection

## Architecture

```
S3 Bucket (JPG Upload)
    ↓
Lambda Function (Go)
    ↓
├── Generate Thumbnails → Upload to S3
├── Extract EXIF data
├── Find related files
└── Store metadata → DynamoDB
```

## Project Status

✅ **Completed**:
- CloudFormation/SAM template with deletion protection for S3 and DynamoDB
- Go Lambda function for thumbnail generation (50px and 400px heights)
- EXIF data extraction and storage
- Related files discovery (same base name, different extensions)
- DynamoDB integration with GUID assignment
- GitHub repository created and initialized
- Deployment scripts (Windows batch and bash)
- Test automation script

⚠️ **Pending**:
- AWS permissions required for deployment (see `AWS_PERMISSIONS_REQUIRED.md`)
- Infrastructure deployment
- End-to-end testing with actual image upload

## Prerequisites

- AWS Account with appropriate permissions (see `AWS_PERMISSIONS_REQUIRED.md` for details)
- Go 1.21+ for building Lambda function
- AWS CLI configured with credentials
- GitHub repository with secrets configured:
  - `AWS_ACCESS_KEY_ID`
  - `AWS_SECRET_ACCESS_KEY`
- GitHub variable:
  - `S3_BUCKET` = `kill-snap-images-759775734231`

## Quick Start

### Option 1: Automated Deployment (GitHub Actions)

**Note**: Requires proper AWS permissions configured as GitHub secrets.

1. Push to `main` branch or trigger workflow manually
2. GitHub Actions will:
   - Build the Go Lambda function
   - Deploy CloudFormation stack
   - Configure S3 bucket protection
   - Set up Lambda trigger

### Option 2: Manual Deployment (Windows)

**Important**: First ensure you have the required AWS permissions documented in `AWS_PERMISSIONS_REQUIRED.md`.

```bash
# Clone the repository
git clone https://github.com/JeremyProffitt/kill-snap.git
cd kill-snap

# Run deployment script
manual-deploy.bat
```

### Option 3: Manual Deployment (Bash/Linux/Mac)

```bash
# Build Lambda function
cd lambda
make build

# Deploy with SAM
sam deploy \
  --template-file template.yaml \
  --stack-name image-thumbnail-stack \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides S3BucketName=your-bucket-name \
  --guided
```

## Testing

### Automated Test Script

Use the provided test script to automatically verify the deployment:

```bash
# Windows
test-deployment.bat

# Linux/Mac
bash test-deployment.sh
```

The test script will:
1. Upload a test JPG image
2. Wait for Lambda processing
3. Verify thumbnails were created (test-image.50.jpg and test-image.400.jpg)
4. Check DynamoDB for metadata records
5. Display Lambda execution logs

### Manual Testing

Upload a JPG image to the S3 bucket:

```bash
# Upload test image
aws s3 cp test-image.jpg s3://kill-snap-images-759775734231/

# Wait a few seconds, then check for thumbnails
aws s3 ls s3://kill-snap-images-759775734231/ --recursive | grep test-image

# Expected files:
# - test-image.jpg (original)
# - test-image.50.jpg (50px high thumbnail)
# - test-image.400.jpg (400px high thumbnail)

# Check DynamoDB for metadata
aws dynamodb scan --table-name ImageMetadata --max-items 5
```

## Resource Protection

The following resources are protected from accidental deletion:

- **S3 Bucket**:
  - DeletionPolicy: Retain
  - Versioning enabled
  - Tagged for lifecycle retention

- **DynamoDB Table**:
  - DeletionPolicy: Retain
  - UpdateReplacePolicy: Retain

Even if the CloudFormation stack is deleted, these resources will remain.

## File Naming Convention

For an original file `image.jpg`, the service creates:
- `image.50.jpg` - 50 pixel high thumbnail
- `image.400.jpg` - 400 pixel high thumbnail

## DynamoDB Schema

```json
{
  "ImageGUID": "uuid-string",
  "OriginalFile": "path/to/image.jpg",
  "Bucket": "bucket-name",
  "Thumbnail50": "path/to/image.50.jpg",
  "Thumbnail400": "path/to/image.400.jpg",
  "RelatedFiles": ["path/to/image.raw", "path/to/image.xmp"],
  "EXIFData": {
    "Make": "Canon",
    "Model": "EOS R5",
    "DateTime": "2024:01:15 10:30:00"
  },
  "Width": 1920,
  "Height": 1080,
  "FileSize": 2048576
}
```

## Development

### Lambda Function

The Lambda function is written in Go and uses:
- `aws-lambda-go` - Lambda runtime
- `aws-sdk-go` - AWS service interactions
- `imaging` - Image resizing
- `goexif` - EXIF data extraction
- `uuid` - GUID generation

### Local Testing

```bash
cd lambda
go test ./...
```

## Troubleshooting

### Lambda not triggering
- Check Lambda execution role permissions
- Verify S3 event notification configuration
- Check CloudWatch Logs for errors

### Thumbnails not appearing
- Verify Lambda has S3 write permissions
- Check Lambda timeout (default: 60s)
- Review CloudWatch Logs for processing errors

### DynamoDB errors
- Verify Lambda has DynamoDB write permissions
- Check table exists and is in ACTIVE state
