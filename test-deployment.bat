@echo off
setlocal enabledelayedexpansion

set BUCKET_NAME=kill-snap-images-759775734231
set REGION=us-east-1
set TEST_IMAGE=test-image.jpg

echo ======================================================
echo Testing Image Thumbnail Service Deployment
echo ======================================================
echo.
echo Bucket: %BUCKET_NAME%
echo Region: %REGION%
echo.

REM Check if we need to create a test image
if not exist %TEST_IMAGE% (
    echo Creating test JPG image...
    python -c "from PIL import Image; img = Image.new('RGB', (800, 600), color='red'); img.save('test-image.jpg')" 2>nul
    if %errorlevel% neq 0 (
        echo [ERROR] Could not create test image. Please provide a test JPG file.
        echo You can download one or create one manually and name it test-image.jpg
        exit /b 1
    )
    echo [OK] Test image created
)

REM Step 1: Upload test image
echo.
echo [1/5] Uploading test image to S3...
aws s3 cp %TEST_IMAGE% s3://%BUCKET_NAME%/test-image.jpg --region %REGION%
if %errorlevel% neq 0 (
    echo [ERROR] Failed to upload image to S3
    echo Please check your AWS credentials and bucket permissions
    exit /b 1
)
echo [OK] Image uploaded successfully

REM Step 2: Wait for Lambda to process
echo.
echo [2/5] Waiting for Lambda to process image (15 seconds)...
timeout /t 15 /nobreak >nul

REM Step 3: Check for thumbnails
echo.
echo [3/5] Checking for generated thumbnails...
aws s3 ls s3://%BUCKET_NAME%/ --recursive --region %REGION% | findstr test-image

set FOUND_50=0
set FOUND_400=0

for /f "tokens=*" %%i in ('aws s3 ls s3://%BUCKET_NAME%/ --recursive --region %REGION%') do (
    echo %%i | findstr /C:"test-image.50.jpg" >nul && set FOUND_50=1
    echo %%i | findstr /C:"test-image.400.jpg" >nul && set FOUND_400=1
)

if %FOUND_50%==1 (
    echo [OK] 50px thumbnail found
) else (
    echo [ERROR] 50px thumbnail NOT found
)

if %FOUND_400%==1 (
    echo [OK] 400px thumbnail found
) else (
    echo [ERROR] 400px thumbnail NOT found
)

REM Step 4: Check DynamoDB
echo.
echo [4/5] Checking DynamoDB for image metadata...
aws dynamodb scan --table-name ImageMetadata --region %REGION% --max-items 5 > dynamodb-output.txt
if %errorlevel% neq 0 (
    echo [ERROR] Failed to scan DynamoDB table
) else (
    echo [OK] DynamoDB scan successful
    echo.
    echo Recent image records:
    type dynamodb-output.txt
    del dynamodb-output.txt
)

REM Step 5: Check Lambda logs
echo.
echo [5/5] Checking Lambda CloudWatch logs...
echo Recent Lambda executions:
aws logs tail /aws/lambda/ImageThumbnailGenerator --since 5m --region %REGION% 2>nul
if %errorlevel% neq 0 (
    echo [WARN] Could not fetch Lambda logs. Function may not have executed yet.
)

REM Summary
echo.
echo ======================================================
echo Test Summary
echo ======================================================
echo.

if %FOUND_50%==1 if %FOUND_400%==1 (
    echo [SUCCESS] All thumbnails generated successfully!
    echo.
    echo Generated files:
    echo - test-image.jpg (original)
    echo - test-image.50.jpg (50px high thumbnail)
    echo - test-image.400.jpg (400px high thumbnail)
    echo.
    echo Download thumbnails to verify:
    echo   aws s3 cp s3://%BUCKET_NAME%/test-image.50.jpg . --region %REGION%
    echo   aws s3 cp s3://%BUCKET_NAME%/test-image.400.jpg . --region %REGION%
) else (
    echo [FAILED] Some thumbnails were not generated.
    echo.
    echo Troubleshooting steps:
    echo 1. Check Lambda function exists:
    echo    aws lambda get-function --function-name ImageThumbnailGenerator --region %REGION%
    echo.
    echo 2. Check S3 event notification:
    echo    aws s3api get-bucket-notification-configuration --bucket %BUCKET_NAME%
    echo.
    echo 3. Check Lambda execution logs:
    echo    aws logs tail /aws/lambda/ImageThumbnailGenerator --follow --region %REGION%
    echo.
    echo 4. Verify Lambda permissions:
    echo    aws iam get-role --role-name ImageThumbnailLambdaRole
    echo.
    echo 5. Test Lambda manually:
    echo    aws lambda invoke --function-name ImageThumbnailGenerator --payload '{"Records":[{"s3":{"bucket":{"name":"%BUCKET_NAME%"},"object":{"key":"test-image.jpg"}}}]}' response.json --region %REGION%
)

echo.
endlocal
