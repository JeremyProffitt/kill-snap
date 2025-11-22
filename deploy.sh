#!/bin/bash
set -e

# Configuration
BUCKET_NAME="${S3_BUCKET:-kill-snap-images-759775734231}"
STACK_NAME="image-thumbnail-stack"
REGION="${AWS_REGION:-us-east-1}"

echo "Deploying Image Thumbnail Service"
echo "Bucket: $BUCKET_NAME"
echo "Stack: $STACK_NAME"
echo "Region: $REGION"

# Build Lambda function
echo "Building Lambda function..."
cd lambda
make clean build
cd ..

# Create S3 bucket for deployment artifacts
DEPLOY_BUCKET="${BUCKET_NAME}-deploy"
echo "Creating deployment bucket: $DEPLOY_BUCKET"
aws s3 mb s3://$DEPLOY_BUCKET --region $REGION 2>/dev/null || echo "Deployment bucket already exists"

# Upload Lambda deployment package
echo "Uploading Lambda function..."
aws s3 cp lambda/deployment.zip s3://$DEPLOY_BUCKET/lambda.zip

# Package CloudFormation template
echo "Packaging CloudFormation template..."
aws cloudformation package \
  --template-file template.yaml \
  --s3-bucket $DEPLOY_BUCKET \
  --output-template-file packaged-template.yaml

# Deploy CloudFormation stack
echo "Deploying CloudFormation stack..."
aws cloudformation deploy \
  --template-file packaged-template.yaml \
  --stack-name $STACK_NAME \
  --capabilities CAPABILITY_IAM \
  --parameter-overrides S3BucketName=$BUCKET_NAME \
  --region $REGION

# Get stack outputs
echo ""
echo "Deployment complete!"
echo ""
aws cloudformation describe-stacks \
  --stack-name $STACK_NAME \
  --query 'Stacks[0].Outputs' \
  --output table \
  --region $REGION
