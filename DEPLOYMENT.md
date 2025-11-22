# Deployment Guide

## Prerequisites

### AWS Permissions Required

The AWS user/role needs the following permissions to deploy this solution:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:CreateBucket",
        "s3:DeleteBucket",
        "s3:PutBucketVersioning",
        "s3:PutBucketNotification",
        "s3:PutBucketTagging",
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket",
        "s3:ListAllMyBuckets"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "lambda:CreateFunction",
        "lambda:UpdateFunctionCode",
        "lambda:UpdateFunctionConfiguration",
        "lambda:DeleteFunction",
        "lambda:GetFunction",
        "lambda:AddPermission",
        "lambda:RemovePermission",
        "lambda:ListFunctions"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:CreateTable",
        "dynamodb:DeleteTable",
        "dynamodb:DescribeTable",
        "dynamodb:UpdateTable",
        "dynamodb:ListTables"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "iam:CreateRole",
        "iam:DeleteRole",
        "iam:GetRole",
        "iam:PassRole",
        "iam:AttachRolePolicy",
        "iam:DetachRolePolicy",
        "iam:PutRolePolicy",
        "iam:DeleteRolePolicy"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "cloudformation:CreateStack",
        "cloudformation:UpdateStack",
        "cloudformation:DeleteStack",
        "cloudformation:DescribeStacks",
        "cloudformation:DescribeStackEvents",
        "cloudformation:ListStacks"
      ],
      "Resource": "*"
    }
  ]
}
```

## Deployment Options

### Option 1: Using AWS CloudFormation (Recommended)

If you have administrator access or the required permissions:

```bash
# Set environment variables
export S3_BUCKET=your-unique-bucket-name
export AWS_REGION=us-east-1

# Build Lambda
cd lambda
make build
cd ..

# Deploy using CloudFormation
./deploy.sh
```

### Option 2: Using AWS SAM CLI

```bash
sam deploy \
  --template-file template.yaml \
  --stack-name image-thumbnail-stack \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides S3BucketName=your-bucket-name \
  --guided
```

### Option 3: Manual Deployment

If you lack CloudFormation permissions, deploy manually:

#### Step 1: Create S3 Bucket

```bash
aws s3 mb s3://your-bucket-name
aws s3api put-bucket-versioning \
  --bucket your-bucket-name \
  --versioning-configuration Status=Enabled
```

#### Step 2: Create DynamoDB Table

```bash
aws dynamodb create-table \
  --table-name ImageMetadata \
  --attribute-definitions AttributeName=ImageGUID,AttributeType=S \
  --key-schema AttributeName=ImageGUID,KeyType=HASH \
  --billing-mode PAY_PER_REQUEST \
  --tags Key=Lifecycle,Value=Retain
```

#### Step 3: Create IAM Role for Lambda

Create a file `trust-policy.json`:
```json
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
```

Create the role:
```bash
aws iam create-role \
  --role-name ImageThumbnailLambdaRole \
  --assume-role-policy-document file://trust-policy.json

# Attach policies
aws iam attach-role-policy \
  --role-name ImageThumbnailLambdaRole \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

# Create custom policy for S3 and DynamoDB
cat > lambda-policy.json <<EOF
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
        "arn:aws:s3:::your-bucket-name",
        "arn:aws:s3:::your-bucket-name/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:Query"
      ],
      "Resource": "arn:aws:dynamodb:*:*:table/ImageMetadata"
    }
  ]
}
EOF

aws iam put-role-policy \
  --role-name ImageThumbnailLambdaRole \
  --policy-name S3DynamoDBAccess \
  --policy-document file://lambda-policy.json
```

#### Step 4: Create Lambda Function

```bash
cd lambda
make build

# Get the role ARN
ROLE_ARN=$(aws iam get-role --role-name ImageThumbnailLambdaRole --query 'Role.Arn' --output text)

# Create Lambda function
aws lambda create-function \
  --function-name ImageThumbnailGenerator \
  --runtime provided.al2023 \
  --role $ROLE_ARN \
  --handler bootstrap \
  --zip-file fileb://deployment.zip \
  --timeout 60 \
  --memory-size 512 \
  --environment "Variables={BUCKET_NAME=your-bucket-name,DYNAMODB_TABLE=ImageMetadata}"
```

#### Step 5: Configure S3 Event Notification

```bash
# Get Lambda ARN
LAMBDA_ARN=$(aws lambda get-function --function-name ImageThumbnailGenerator --query 'Configuration.FunctionArn' --output text)

# Add Lambda permission
aws lambda add-permission \
  --function-name ImageThumbnailGenerator \
  --statement-id s3-trigger \
  --action lambda:InvokeFunction \
  --principal s3.amazonaws.com \
  --source-arn arn:aws:s3:::your-bucket-name

# Configure S3 notification
cat > notification.json <<EOF
{
  "LambdaFunctionConfigurations": [
    {
      "LambdaFunctionArn": "$LAMBDA_ARN",
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
EOF

aws s3api put-bucket-notification-configuration \
  --bucket your-bucket-name \
  --notification-configuration file://notification.json
```

### Option 4: Using GitHub Actions

1. Configure GitHub secrets:
   - `AWS_ACCESS_KEY_ID`
   - `AWS_SECRET_ACCESS_KEY`

2. Configure GitHub variable:
   - `S3_BUCKET` - your bucket name

3. Push to main branch - the workflow will deploy automatically

## Testing the Deployment

### Upload a test image

```bash
# Create or download a test JPG image
aws s3 cp test.jpg s3://your-bucket-name/

# Wait a few seconds for processing
sleep 10

# Check for thumbnails
aws s3 ls s3://your-bucket-name/ --recursive | grep test

# Expected output:
# test.jpg
# test.50.jpg
# test.400.jpg
```

### Verify DynamoDB Record

```bash
aws dynamodb scan --table-name ImageMetadata --max-items 1
```

### Check Lambda Logs

```bash
aws logs tail /aws/lambda/ImageThumbnailGenerator --follow
```

## Troubleshooting

### Lambda Not Triggering

```bash
# Check Lambda permissions
aws lambda get-policy --function-name ImageThumbnailGenerator

# Check S3 notification configuration
aws s3api get-bucket-notification-configuration --bucket your-bucket-name

# Check CloudWatch Logs
aws logs tail /aws/lambda/ImageThumbnailGenerator --since 30m
```

### Permission Errors

Ensure the Lambda execution role has:
- S3 read/write permissions for the bucket
- DynamoDB write permissions for ImageMetadata table
- CloudWatch Logs write permissions

### Thumbnails Not Generated

Check Lambda CloudWatch Logs for errors:
```bash
aws logs filter-log-events \
  --log-group-name /aws/lambda/ImageThumbnailGenerator \
  --filter-pattern "ERROR"
```

## Cleanup

To remove all resources (CAUTION: This will NOT delete the S3 bucket or DynamoDB table due to retention policies):

```bash
# Delete CloudFormation stack
aws cloudformation delete-stack --stack-name image-thumbnail-stack

# Manually delete S3 bucket (if desired)
aws s3 rb s3://your-bucket-name --force

# Manually delete DynamoDB table (if desired)
aws dynamodb delete-table --table-name ImageMetadata
```
