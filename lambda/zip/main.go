package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	maxZipSize = 4 * 1024 * 1024 * 1024 // 4GB pre-zip size limit
)

var (
	bucketName   string
	imageTable   string
	projectTable string
	s3Client     *s3.S3
	s3Uploader   *s3manager.Uploader
	ddbClient    *dynamodb.DynamoDB
)

// ZipRequest is the event payload for triggering zip generation
type ZipRequest struct {
	ProjectID string `json:"projectId"`
}

// Project represents a project record in DynamoDB
type Project struct {
	ProjectID        string    `json:"projectId" dynamodbav:"ProjectID"`
	Name             string    `json:"name" dynamodbav:"Name"`
	S3Prefix         string    `json:"s3Prefix,omitempty" dynamodbav:"S3Prefix,omitempty"`
	CreatedAt        string    `json:"createdAt" dynamodbav:"CreatedAt"`
	ImageCount       int       `json:"imageCount" dynamodbav:"ImageCount"`
	CatalogPath      string    `json:"catalogPath,omitempty" dynamodbav:"CatalogPath,omitempty"`
	CatalogUpdatedAt string    `json:"catalogUpdatedAt,omitempty" dynamodbav:"CatalogUpdatedAt,omitempty"`
	ZipFiles         []ZipFile `json:"zipFiles,omitempty" dynamodbav:"ZipFiles,omitempty"`
}

// ZipFile represents a generated zip file
type ZipFile struct {
	Key        string `json:"key" dynamodbav:"Key"`
	Size       int64  `json:"size" dynamodbav:"Size"`
	ImageCount int    `json:"imageCount" dynamodbav:"ImageCount"`
	CreatedAt  string `json:"createdAt" dynamodbav:"CreatedAt"`
	Status     string `json:"status" dynamodbav:"Status"`
}

// ImageRecord represents an image in DynamoDB
type ImageRecord struct {
	ImageGUID    string `json:"imageGUID" dynamodbav:"ImageGUID"`
	OriginalFile string `json:"originalFile" dynamodbav:"OriginalFile"`
	FileSize     int64  `json:"fileSize" dynamodbav:"FileSize"`
	ProjectID    string `json:"projectId,omitempty" dynamodbav:"ProjectID,omitempty"`
}

func init() {
	bucketName = os.Getenv("BUCKET_NAME")
	imageTable = os.Getenv("IMAGE_TABLE")
	projectTable = os.Getenv("PROJECT_TABLE")

	sess := session.Must(session.NewSession())
	s3Client = s3.New(sess)
	s3Uploader = s3manager.NewUploader(sess)
	ddbClient = dynamodb.New(sess)
}

func getProjectS3Prefix(project Project) string {
	if project.S3Prefix != "" {
		return project.S3Prefix
	}
	return project.ProjectID
}

func handleRequest(ctx context.Context, request ZipRequest) error {
	fmt.Printf("=== ZIP GENERATION STARTED ===\n")
	fmt.Printf("Project ID: %s\n", request.ProjectID)
	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Image Table: %s\n", imageTable)
	fmt.Printf("Project Table: %s\n", projectTable)

	// Get project details
	fmt.Printf("Fetching project details from DynamoDB...\n")
	projectResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(request.ProjectID)},
		},
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to get project from DynamoDB: %v\n", err)
		return fmt.Errorf("project not found: %s", request.ProjectID)
	}
	if projectResult.Item == nil {
		fmt.Printf("ERROR: Project not found in DynamoDB: %s\n", request.ProjectID)
		return fmt.Errorf("project not found: %s", request.ProjectID)
	}

	var project Project
	if err := dynamodbattribute.UnmarshalMap(projectResult.Item, &project); err != nil {
		fmt.Printf("ERROR: Failed to unmarshal project: %v\n", err)
		return fmt.Errorf("failed to unmarshal project: %v", err)
	}
	fmt.Printf("Project found: Name='%s', ImageCount=%d, S3Prefix='%s'\n", project.Name, project.ImageCount, project.S3Prefix)

	// Query all images in this project
	fmt.Printf("Querying images for project using ProjectIndex...\n")
	images, err := getProjectImages(request.ProjectID)
	if err != nil {
		fmt.Printf("ERROR: Failed to get project images: %v\n", err)
		return fmt.Errorf("failed to get project images: %v", err)
	}

	if len(images) == 0 {
		fmt.Printf("WARNING: No images in project, nothing to zip\n")
		return nil
	}

	fmt.Printf("Found %d images to zip\n", len(images))
	for i, img := range images {
		fmt.Printf("  [%d] ImageGUID=%s, File=%s, Size=%d bytes\n", i+1, img.ImageGUID, img.OriginalFile, img.FileSize)
	}

	// Sort images by file path for consistent ordering
	sort.Slice(images, func(i, j int) bool {
		return images[i].OriginalFile < images[j].OriginalFile
	})

	// Split images into batches based on 4GB pre-zip size limit
	batches := splitIntoBatches(images)
	fmt.Printf("Split into %d zip batch(es)\n", len(batches))

	// Get project S3 prefix
	s3Prefix := getProjectS3Prefix(project)
	dateStr := time.Now().Format("2006-01-02")
	sanitizedName := sanitizeZipName(project.Name)

	var zipFiles []ZipFile

	for i, batch := range batches {
		var zipKey string
		// Store zips in root of project folder (projects/{s3Prefix}/)
		if len(batches) == 1 {
			zipKey = fmt.Sprintf("projects/%s/%s_%s.zip", s3Prefix, sanitizedName, dateStr)
		} else {
			zipKey = fmt.Sprintf("projects/%s/%s_%s_part%d.zip", s3Prefix, sanitizedName, dateStr, i+1)
		}

		// Create zip file (include lrcat catalog if available)
		zipInfo, err := createAndUploadZip(ctx, batch, zipKey, project.CatalogPath)
		if err != nil {
			fmt.Printf("Error creating zip %s: %v\n", zipKey, err)
			// Record failed zip
			zipFiles = append(zipFiles, ZipFile{
				Key:        zipKey,
				Size:       0,
				ImageCount: len(batch),
				CreatedAt:  time.Now().Format(time.RFC3339),
				Status:     "failed",
			})
			continue
		}

		zipFiles = append(zipFiles, *zipInfo)
		fmt.Printf("Created zip: %s (%d bytes, %d images)\n", zipKey, zipInfo.Size, zipInfo.ImageCount)
	}

	// Update project with zip files info
	if err := updateProjectZipFiles(request.ProjectID, zipFiles); err != nil {
		return fmt.Errorf("failed to update project zip files: %v", err)
	}

	fmt.Printf("Zip generation complete for project: %s\n", request.ProjectID)
	return nil
}

func getProjectImages(projectID string) ([]ImageRecord, error) {
	var images []ImageRecord

	fmt.Printf("Querying DynamoDB table '%s' with index 'ProjectIndex' for ProjectID='%s'\n", imageTable, projectID)

	input := &dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectIndex"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
	}

	pageCount := 0
	err := ddbClient.QueryPages(input, func(page *dynamodb.QueryOutput, lastPage bool) bool {
		pageCount++
		fmt.Printf("Processing query page %d, items in page: %d, lastPage: %v\n", pageCount, len(page.Items), lastPage)
		for _, item := range page.Items {
			var img ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &img); err == nil {
				images = append(images, img)
			} else {
				fmt.Printf("WARNING: Failed to unmarshal image record: %v\n", err)
			}
		}
		return !lastPage
	})

	if err != nil {
		fmt.Printf("ERROR: DynamoDB query failed: %v\n", err)
	} else {
		fmt.Printf("Query complete. Total images retrieved: %d\n", len(images))
	}

	return images, err
}

func splitIntoBatches(images []ImageRecord) [][]ImageRecord {
	fmt.Printf("Splitting %d images into batches (max %d bytes per batch)\n", len(images), maxZipSize)

	var batches [][]ImageRecord
	var currentBatch []ImageRecord
	var currentSize int64

	for _, img := range images {
		// If adding this image would exceed limit, start new batch
		if currentSize+img.FileSize > maxZipSize && len(currentBatch) > 0 {
			fmt.Printf("  Batch %d complete: %d images, %d bytes\n", len(batches)+1, len(currentBatch), currentSize)
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentSize = 0
		}

		currentBatch = append(currentBatch, img)
		currentSize += img.FileSize
	}

	// Add the last batch
	if len(currentBatch) > 0 {
		fmt.Printf("  Batch %d complete: %d images, %d bytes\n", len(batches)+1, len(currentBatch), currentSize)
		batches = append(batches, currentBatch)
	}

	fmt.Printf("Total batches created: %d\n", len(batches))
	return batches
}

func sanitizeZipName(name string) string {
	// Replace spaces and special characters with underscores
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")

	// Keep only alphanumeric and underscores
	var sanitized strings.Builder
	for _, r := range result {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sanitized.WriteRune(r)
		}
	}

	result = sanitized.String()

	// Remove consecutive underscores
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	result = strings.Trim(result, "_")

	if result == "" {
		result = "project"
	}

	// Limit length
	if len(result) > 50 {
		result = result[:50]
	}

	return result
}

func createAndUploadZip(ctx context.Context, images []ImageRecord, zipKey string, catalogPath string) (*ZipFile, error) {
	fmt.Printf("=== Creating zip: %s ===\n", zipKey)
	fmt.Printf("Images to include: %d\n", len(images))
	fmt.Printf("Catalog path: %s\n", catalogPath)

	// Create a temporary file for the zip
	tmpFile, err := os.CreateTemp("", "project-*.zip")
	if err != nil {
		fmt.Printf("ERROR: Failed to create temp file: %v\n", err)
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	fmt.Printf("Created temp file: %s\n", tmpPath)
	defer os.Remove(tmpPath)

	// Create zip writer
	zipWriter := zip.NewWriter(tmpFile)

	// Track file names to avoid duplicates
	fileNames := make(map[string]int)
	successCount := 0
	failCount := 0

	// Helper function to add a file to zip using Store method (no compression)
	addFileToZip := func(s3Key string, zipFileName string) error {
		fmt.Printf("  Downloading from S3: bucket=%s, key=%s\n", bucketName, s3Key)
		getResult, err := s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(s3Key),
		})
		if err != nil {
			return fmt.Errorf("failed to download from S3: %v", err)
		}
		defer getResult.Body.Close()

		// Get file size for the header
		var fileSize int64
		if getResult.ContentLength != nil {
			fileSize = *getResult.ContentLength
		}

		// Create zip header with Store method (no compression)
		header := &zip.FileHeader{
			Name:   zipFileName,
			Method: zip.Store, // Store mode - no compression
		}
		header.SetModTime(time.Now())
		// Set uncompressed size for store method
		header.UncompressedSize64 = uint64(fileSize)

		fmt.Printf("  Adding to zip as: %s (Store mode, size: %d bytes)\n", zipFileName, fileSize)
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %v", err)
		}

		bytesWritten, err := io.Copy(writer, getResult.Body)
		if err != nil {
			return fmt.Errorf("failed to write to zip: %v", err)
		}

		fmt.Printf("  SUCCESS: Written %d bytes\n", bytesWritten)
		return nil
	}

	// Add lrcat catalog file first if available
	if catalogPath != "" {
		fmt.Printf("Adding catalog file to zip: %s\n", catalogPath)
		catalogFileName := filepath.Base(catalogPath)
		if err := addFileToZip(catalogPath, catalogFileName); err != nil {
			fmt.Printf("  WARNING: Failed to add catalog to zip: %v\n", err)
		} else {
			fileNames[catalogFileName] = 1
			successCount++
		}
	}

	for i, img := range images {
		fmt.Printf("[%d/%d] Processing: %s\n", i+1, len(images), img.OriginalFile)

		// Get unique filename
		baseName := filepath.Base(img.OriginalFile)
		fileName := baseName
		if count, exists := fileNames[baseName]; exists {
			ext := filepath.Ext(baseName)
			name := strings.TrimSuffix(baseName, ext)
			fileName = fmt.Sprintf("%s_%d%s", name, count+1, ext)
			fileNames[baseName] = count + 1
			fmt.Printf("  Renamed to avoid duplicate: %s\n", fileName)
		} else {
			fileNames[baseName] = 1
		}

		if err := addFileToZip(img.OriginalFile, fileName); err != nil {
			fmt.Printf("  ERROR: %v\n", err)
			failCount++
			continue
		}
		successCount++
	}

	fmt.Printf("Zip content complete. Success: %d, Failed: %d\n", successCount, failCount)

	// Close zip writer
	fmt.Printf("Closing zip writer...\n")
	if err := zipWriter.Close(); err != nil {
		tmpFile.Close()
		fmt.Printf("ERROR: Failed to close zip writer: %v\n", err)
		return nil, fmt.Errorf("failed to close zip writer: %v", err)
	}
	tmpFile.Close()

	// Get zip file size
	stat, err := os.Stat(tmpPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to stat zip file: %v\n", err)
		return nil, fmt.Errorf("failed to stat zip file: %v", err)
	}
	fmt.Printf("Zip file size: %d bytes\n", stat.Size())

	// Upload to S3
	fmt.Printf("Uploading zip to S3: bucket=%s, key=%s\n", bucketName, zipKey)
	uploadFile, err := os.Open(tmpPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to open zip for upload: %v\n", err)
		return nil, fmt.Errorf("failed to open zip for upload: %v", err)
	}
	defer uploadFile.Close()

	_, err = s3Uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(zipKey),
		Body:        uploadFile,
		ContentType: aws.String("application/zip"),
	})
	if err != nil {
		fmt.Printf("ERROR: Failed to upload zip to S3: %v\n", err)
		return nil, fmt.Errorf("failed to upload zip: %v", err)
	}

	fmt.Printf("SUCCESS: Zip uploaded to S3: %s\n", zipKey)

	return &ZipFile{
		Key:        zipKey,
		Size:       stat.Size(),
		ImageCount: successCount,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Status:     "complete",
	}, nil
}

func updateProjectZipFiles(projectID string, zipFiles []ZipFile) error {
	fmt.Printf("Updating project %s with %d zip file(s)...\n", projectID, len(zipFiles))
	for i, zf := range zipFiles {
		fmt.Printf("  [%d] Key=%s, Size=%d, ImageCount=%d, Status=%s\n", i+1, zf.Key, zf.Size, zf.ImageCount, zf.Status)
	}

	// Marshal zip files to DynamoDB format
	zipFilesList, err := dynamodbattribute.MarshalList(zipFiles)
	if err != nil {
		fmt.Printf("ERROR: Failed to marshal zip files: %v\n", err)
		return err
	}

	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ZipFiles = :zips"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":zips": {L: zipFilesList},
		},
	})

	if err != nil {
		fmt.Printf("ERROR: Failed to update project zip files: %v\n", err)
	} else {
		fmt.Printf("SUCCESS: Project zip files updated\n")
	}

	return err
}

func main() {
	lambda.Start(handleRequest)
}
