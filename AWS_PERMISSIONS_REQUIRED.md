# AWS Permissions Required for Deployment

## Current Status

The AWS user `arn:aws:iam::759775734231:user/rog-laptop` currently lacks the necessary permissions to deploy this solution automatically.

## Missing Permissions

The following AWS permissions are required to deploy the image thumbnail service:

### S3 Permissions
```json
{
  "Effect": "Allow",
  "Action": [
    "s3:CreateBucket",
    "s3:PutBucketVersioning",
    "s3:PutBucketNotificationConfiguration",
    "s3:PutBucketTagging",
    "s3:PutObject",
    "s3:GetObject",
    "s3:ListBucket"
  ],
  "Resource": [
    "arn:aws:s3:::ai-template-images-759775734231",
    "arn:aws:s3:::ai-template-images-759775734231/*"
  ]
}
```

### DynamoDB Permissions
```json
{
  "Effect": "Allow",
  "Action": [
    "dynamodb:CreateTable",
    "dynamodb:DescribeTable",
    "dynamodb:UpdateTable",
    "dynamodb:PutItem",
    "dynamodb:GetItem",
    "dynamodb:Query",
    "dynamodb:Scan",
    "dynamodb:TagResource"
  ],
  "Resource": "arn:aws:dynamodb:us-east-1:759775734231:table/ImageMetadata"
}
```

### IAM Permissions
```json
{
  "Effect": "Allow",
  "Action": [
    "iam:CreateRole",
    "iam:GetRole",
    "iam:PassRole",
    "iam:AttachRolePolicy",
    "iam:PutRolePolicy"
  ],
  "Resource": "arn:aws:iam::759775734231:role/ImageThumbnailLambdaRole"
}
```

### Lambda Permissions
```json
{
  "Effect": "Allow",
  "Action": [
    "lambda:CreateFunction",
    "lambda:UpdateFunctionCode",
    "lambda:UpdateFunctionConfiguration",
    "lambda:GetFunction",
    "lambda:AddPermission",
    "lambda:ListFunctions"
  ],
  "Resource": "arn:aws:lambda:us-east-1:759775734231:function:ImageThumbnailGenerator"
}
```

## Complete IAM Policy

Here's the complete IAM policy that can be attached to the user:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "S3Permissions",
      "Effect": "Allow",
      "Action": [
        "s3:CreateBucket",
        "s3:PutBucketVersioning",
        "s3:PutBucketNotificationConfiguration",
        "s3:PutBucketTagging",
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::ai-template-images-759775734231",
        "arn:aws:s3:::ai-template-images-759775734231/*"
      ]
    },
    {
      "Sid": "DynamoDBPermissions",
      "Effect": "Allow",
      "Action": [
        "dynamodb:CreateTable",
        "dynamodb:DescribeTable",
        "dynamodb:UpdateTable",
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:Scan",
        "dynamodb:TagResource"
      ],
      "Resource": "arn:aws:dynamodb:us-east-1:759775734231:table/ImageMetadata"
    },
    {
      "Sid": "IAMPermissions",
      "Effect": "Allow",
      "Action": [
        "iam:CreateRole",
        "iam:GetRole",
        "iam:PassRole",
        "iam:AttachRolePolicy",
        "iam:PutRolePolicy"
      ],
      "Resource": "arn:aws:iam::759775734231:role/ImageThumbnailLambdaRole"
    },
    {
      "Sid": "LambdaPermissions",
      "Effect": "Allow",
      "Action": [
        "lambda:CreateFunction",
        "lambda:UpdateFunctionCode",
        "lambda:UpdateFunctionConfiguration",
        "lambda:GetFunction",
        "lambda:AddPermission",
        "lambda:ListFunctions"
      ],
      "Resource": "arn:aws:lambda:us-east-1:759775734231:function:ImageThumbnailGenerator"
    },
    {
      "Sid": "CloudWatchLogsPermissions",
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams"
      ],
      "Resource": "arn:aws:logs:us-east-1:759775734231:*"
    }
  ]
}
```

## How to Apply These Permissions

### Option 1: Using AWS Console

1. Log in to AWS Console as an administrator
2. Go to IAM → Users → rog-laptop
3. Click "Add permissions" → "Attach policies directly"
4. Click "Create policy"
5. Paste the JSON policy above
6. Name it `ImageThumbnailServiceDeploymentPolicy`
7. Attach it to the user

### Option 2: Using AWS CLI (as administrator)

```bash
# Save the policy to a file
cat > image-thumbnail-policy.json <<'EOF'
[paste the complete policy from above]
EOF

# Create the policy
aws iam create-policy \
  --policy-name ImageThumbnailServiceDeploymentPolicy \
  --policy-document file://image-thumbnail-policy.json

# Attach to user
aws iam attach-user-policy \
  --user-name rog-laptop \
  --policy-arn arn:aws:iam::759775734231:policy/ImageThumbnailServiceDeploymentPolicy
```

## Alternative: Use an Administrator Account

If you have access to an AWS account with administrator privileges, you can:

1. Configure AWS CLI with admin credentials:
   ```bash
   aws configure --profile admin
   ```

2. Run the deployment script with the admin profile:
   ```bash
   AWS_PROFILE=admin ./manual-deploy.bat
   ```

## After Permissions Are Applied

Once the permissions are applied, run the deployment script:

```bash
cd d:\dev\ai-template
manual-deploy.bat
```

The script will create:
- S3 bucket: `ai-template-images-759775734231`
- DynamoDB table: `ImageMetadata`
- Lambda function: `ImageThumbnailGenerator`
- IAM role: `ImageThumbnailLambdaRole`
- S3 event notification to trigger the Lambda

## Verification

After deployment, verify each component:

```bash
# Check S3 bucket
aws s3 ls s3://ai-template-images-759775734231

# Check DynamoDB table
aws dynamodb describe-table --table-name ImageMetadata

# Check Lambda function
aws lambda get-function --function-name ImageThumbnailGenerator

# Check IAM role
aws iam get-role --role-name ImageThumbnailLambdaRole
```
