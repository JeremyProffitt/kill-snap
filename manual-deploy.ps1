# PowerShell deployment script for Windows
param(
    [string]$BucketName = "kill-snap-images-759775734231",
    [string]$Region = "us-east-1"
)

$ErrorActionPreference = "Stop"

Write-Host "Manual Deployment Script for Image Thumbnail Service" -ForegroundColor Cyan
Write-Host "======================================================" -ForegroundColor Cyan
Write-Host ""

# Step 1: Create S3 Bucket
Write-Host "[1/6] Creating S3 Bucket: $BucketName" -ForegroundColor Yellow
try {
    aws s3 mb "s3://$BucketName" --region $Region 2>$null
    Write-Host "✓ Bucket created successfully" -ForegroundColor Green
} catch {
    Write-Host "⚠ Bucket may already exist or access denied. Continuing..." -ForegroundColor Yellow
}

# Enable versioning
Write-Host "  Enabling versioning..." -ForegroundColor Yellow
aws s3api put-bucket-versioning --bucket $BucketName --versioning-configuration Status=Enabled

# Step 2: Create DynamoDB Table
Write-Host "`n[2/6] Creating DynamoDB Table: ImageMetadata" -ForegroundColor Yellow
try {
    aws dynamodb create-table `
        --table-name ImageMetadata `
        --attribute-definitions AttributeName=ImageGUID,AttributeType=S `
        --key-schema AttributeName=ImageGUID,KeyType=HASH `
        --billing-mode PAY_PER_REQUEST `
        --tags Key=Lifecycle,Value=Retain `
        --region $Region
    Write-Host "✓ DynamoDB table created successfully" -ForegroundColor Green
} catch {
    Write-Host "⚠ Table may already exist. Continuing..." -ForegroundColor Yellow
}

# Wait for table to be active
Write-Host "  Waiting for table to be active..." -ForegroundColor Yellow
aws dynamodb wait table-exists --table-name ImageMetadata --region $Region

# Step 3: Create IAM Role
Write-Host "`n[3/6] Creating IAM Role for Lambda" -ForegroundColor Yellow

# Create trust policy
$trustPolicy = @"
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
"@

Set-Content -Path "trust-policy.json" -Value $trustPolicy

try {
    aws iam create-role `
        --role-name ImageThumbnailLambdaRole `
        --assume-role-policy-document file://trust-policy.json
    Write-Host "✓ IAM role created" -ForegroundColor Green
} catch {
    Write-Host "⚠ Role may already exist. Continuing..." -ForegroundColor Yellow
}

# Attach basic execution policy
aws iam attach-role-policy `
    --role-name ImageThumbnailLambdaRole `
    --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Get account ID
$accountId = (aws sts get-caller-identity --query Account --output text)

# Create custom policy
$customPolicy = @"
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:GetObject",
        "s3:PutObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::$BucketName",
        "arn:aws:s3:::$BucketName/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:Scan"
      ],
      "Resource": "arn:aws:dynamodb:${Region}:${accountId}:table/ImageMetadata"
    }
  ]
}
"@

Set-Content -Path "lambda-policy.json" -Value $customPolicy

aws iam put-role-policy `
    --role-name ImageThumbnailLambdaRole `
    --policy-name S3DynamoDBAccess `
    --policy-document file://lambda-policy.json

Write-Host "  Waiting for role to propagate..." -ForegroundColor Yellow
Start-Sleep -Seconds 10

# Step 4: Build and Deploy Lambda
Write-Host "`n[4/6] Building Lambda Function" -ForegroundColor Yellow
Set-Location lambda
if (Test-Path bootstrap) { Remove-Item bootstrap }
if (Test-Path deployment.zip) { Remove-Item deployment.zip }

$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -tags lambda.norpc -o bootstrap main.go

python -m zipfile -c deployment.zip bootstrap
Write-Host "✓ Lambda function built" -ForegroundColor Green

Set-Location ..

# Get role ARN
$roleArn = (aws iam get-role --role-name ImageThumbnailLambdaRole --query 'Role.Arn' --output text)

Write-Host "`n[5/6] Creating Lambda Function" -ForegroundColor Yellow
try {
    aws lambda create-function `
        --function-name ImageThumbnailGenerator `
        --runtime provided.al2023 `
        --role $roleArn `
        --handler bootstrap `
        --zip-file fileb://lambda/deployment.zip `
        --timeout 60 `
        --memory-size 512 `
        --environment "Variables={BUCKET_NAME=$BucketName,DYNAMODB_TABLE=ImageMetadata}" `
        --region $Region
    Write-Host "✓ Lambda function created" -ForegroundColor Green
} catch {
    Write-Host "⚠ Function may already exist. Updating code..." -ForegroundColor Yellow
    aws lambda update-function-code `
        --function-name ImageThumbnailGenerator `
        --zip-file fileb://lambda/deployment.zip `
        --region $Region

    aws lambda update-function-configuration `
        --function-name ImageThumbnailGenerator `
        --environment "Variables={BUCKET_NAME=$BucketName,DYNAMODB_TABLE=ImageMetadata}" `
        --region $Region
}

# Step 6: Configure S3 Trigger
Write-Host "`n[6/6] Configuring S3 Event Trigger" -ForegroundColor Yellow

$lambdaArn = (aws lambda get-function --function-name ImageThumbnailGenerator --query 'Configuration.FunctionArn' --output text --region $Region)

# Add permission
try {
    aws lambda add-permission `
        --function-name ImageThumbnailGenerator `
        --statement-id s3-trigger `
        --action lambda:InvokeFunction `
        --principal s3.amazonaws.com `
        --source-arn "arn:aws:s3:::$BucketName" `
        --region $Region
} catch {
    Write-Host "⚠ Permission may already exist. Continuing..." -ForegroundColor Yellow
}

# Configure notification
$notification = @"
{
  "LambdaFunctionConfigurations": [
    {
      "LambdaFunctionArn": "$lambdaArn",
      "Events": ["s3:ObjectCreated:*"],
      "Filter": {
        "Key": {
          "FilterRules": [
            {
              "Name": "suffix",
              "Value": ".jpg"
            }
          ]
        }
      }
    }
  ]
}
"@

Set-Content -Path "notification.json" -Value $notification

aws s3api put-bucket-notification-configuration `
    --bucket $BucketName `
    --notification-configuration file://notification.json

Write-Host "✓ S3 trigger configured" -ForegroundColor Green

# Cleanup temp files
Remove-Item trust-policy.json -ErrorAction SilentlyContinue
Remove-Item lambda-policy.json -ErrorAction SilentlyContinue
Remove-Item notification.json -ErrorAction SilentlyContinue

Write-Host "`n======================================================" -ForegroundColor Cyan
Write-Host "Deployment Complete!" -ForegroundColor Green
Write-Host "======================================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "S3 Bucket: $BucketName" -ForegroundColor Cyan
Write-Host "DynamoDB Table: ImageMetadata" -ForegroundColor Cyan
Write-Host "Lambda Function: ImageThumbnailGenerator" -ForegroundColor Cyan
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Upload a JPG image to test: aws s3 cp test.jpg s3://$BucketName/" -ForegroundColor White
Write-Host "2. Check for thumbnails: aws s3 ls s3://$BucketName/ --recursive" -ForegroundColor White
Write-Host "3. View DynamoDB records: aws dynamodb scan --table-name ImageMetadata" -ForegroundColor White
Write-Host ""
