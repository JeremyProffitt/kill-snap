@echo off
setlocal enabledelayedexpansion

set BUCKET_NAME=kill-snap-images-759775734231
set REGION=us-east-1

echo ======================================================
echo Manual Deployment Script for Image Thumbnail Service
echo ======================================================
echo.

REM Step 1: Create S3 Bucket
echo [1/6] Creating S3 Bucket: %BUCKET_NAME%
aws s3 mb s3://%BUCKET_NAME% --region %REGION% 2>nul
if %errorlevel% equ 0 (
    echo [OK] Bucket created successfully
) else (
    echo [WARN] Bucket may already exist. Continuing...
)

echo   Enabling versioning...
aws s3api put-bucket-versioning --bucket %BUCKET_NAME% --versioning-configuration Status=Enabled

REM Step 2: Create DynamoDB Table
echo.
echo [2/6] Creating DynamoDB Table: ImageMetadata
aws dynamodb create-table --table-name ImageMetadata --attribute-definitions AttributeName=ImageGUID,AttributeType=S --key-schema AttributeName=ImageGUID,KeyType=HASH --billing-mode PAY_PER_REQUEST --tags Key=Lifecycle,Value=Retain --region %REGION% 2>nul
if %errorlevel% equ 0 (
    echo [OK] DynamoDB table created successfully
) else (
    echo [WARN] Table may already exist. Continuing...
)

echo   Waiting for table to be active...
aws dynamodb wait table-exists --table-name ImageMetadata --region %REGION%

REM Step 3: Create IAM Role
echo.
echo [3/6] Creating IAM Role for Lambda

REM Create trust policy
(
echo {
echo   "Version": "2012-10-17",
echo   "Statement": [{
echo     "Effect": "Allow",
echo     "Principal": {"Service": "lambda.amazonaws.com"},
echo     "Action": "sts:AssumeRole"
echo   }]
echo }
) > trust-policy.json

aws iam create-role --role-name ImageThumbnailLambdaRole --assume-role-policy-document file://trust-policy.json 2>nul
if %errorlevel% equ 0 (
    echo [OK] IAM role created
) else (
    echo [WARN] Role may already exist. Continuing...
)

aws iam attach-role-policy --role-name ImageThumbnailLambdaRole --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

REM Get account ID
for /f %%i in ('aws sts get-caller-identity --query Account --output text') do set ACCOUNT_ID=%%i

REM Create custom policy
(
echo {
echo   "Version": "2012-10-17",
echo   "Statement": [{
echo     "Effect": "Allow",
echo     "Action": ["s3:GetObject", "s3:PutObject", "s3:ListBucket"],
echo     "Resource": ["arn:aws:s3:::%BUCKET_NAME%", "arn:aws:s3:::%BUCKET_NAME%/*"]
echo   }, {
echo     "Effect": "Allow",
echo     "Action": ["dynamodb:PutItem", "dynamodb:GetItem", "dynamodb:Query", "dynamodb:Scan"],
echo     "Resource": "arn:aws:dynamodb:%REGION%:%ACCOUNT_ID%:table/ImageMetadata"
echo   }]
echo }
) > lambda-policy.json

aws iam put-role-policy --role-name ImageThumbnailLambdaRole --policy-name S3DynamoDBAccess --policy-document file://lambda-policy.json

echo   Waiting for role to propagate...
timeout /t 10 /nobreak >nul

REM Step 4: Build Lambda
echo.
echo [4/6] Building Lambda Function
cd lambda
if exist bootstrap del bootstrap
if exist deployment.zip del deployment.zip

set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -tags lambda.norpc -o bootstrap main.go

python -m zipfile -c deployment.zip bootstrap
echo [OK] Lambda function built

cd ..

REM Step 5: Create Lambda Function
echo.
echo [5/6] Creating Lambda Function

for /f %%i in ('aws iam get-role --role-name ImageThumbnailLambdaRole --query Role.Arn --output text') do set ROLE_ARN=%%i

aws lambda create-function --function-name ImageThumbnailGenerator --runtime provided.al2023 --role %ROLE_ARN% --handler bootstrap --zip-file fileb://lambda/deployment.zip --timeout 60 --memory-size 512 --environment Variables={BUCKET_NAME=%BUCKET_NAME%,DYNAMODB_TABLE=ImageMetadata} --region %REGION% 2>nul
if %errorlevel% equ 0 (
    echo [OK] Lambda function created
) else (
    echo [WARN] Function may already exist. Updating...
    aws lambda update-function-code --function-name ImageThumbnailGenerator --zip-file fileb://lambda/deployment.zip --region %REGION%
    aws lambda update-function-configuration --function-name ImageThumbnailGenerator --environment Variables={BUCKET_NAME=%BUCKET_NAME%,DYNAMODB_TABLE=ImageMetadata} --region %REGION%
)

REM Step 6: Configure S3 Trigger
echo.
echo [6/6] Configuring S3 Event Trigger

for /f %%i in ('aws lambda get-function --function-name ImageThumbnailGenerator --query Configuration.FunctionArn --output text --region %REGION%') do set LAMBDA_ARN=%%i

aws lambda add-permission --function-name ImageThumbnailGenerator --statement-id s3-trigger --action lambda:InvokeFunction --principal s3.amazonaws.com --source-arn arn:aws:s3:::%BUCKET_NAME% --region %REGION% 2>nul

REM Create notification config
(
echo {
echo   "LambdaFunctionConfigurations": [{
echo     "LambdaFunctionArn": "%LAMBDA_ARN%",
echo     "Events": ["s3:ObjectCreated:*"],
echo     "Filter": {
echo       "Key": {
echo         "FilterRules": [{
echo           "Name": "suffix",
echo           "Value": ".jpg"
echo         }]
echo       }
echo     }
echo   }]
echo }
) > notification.json

aws s3api put-bucket-notification-configuration --bucket %BUCKET_NAME% --notification-configuration file://notification.json

echo [OK] S3 trigger configured

REM Cleanup
del trust-policy.json 2>nul
del lambda-policy.json 2>nul
del notification.json 2>nul

echo.
echo ======================================================
echo Deployment Complete!
echo ======================================================
echo.
echo S3 Bucket: %BUCKET_NAME%
echo DynamoDB Table: ImageMetadata
echo Lambda Function: ImageThumbnailGenerator
echo.
echo Next steps:
echo 1. Upload a JPG image to test: aws s3 cp test.jpg s3://%BUCKET_NAME%/
echo 2. Check for thumbnails: aws s3 ls s3://%BUCKET_NAME%/ --recursive
echo 3. View DynamoDB records: aws dynamodb scan --table-name ImageMetadata
echo.

endlocal
