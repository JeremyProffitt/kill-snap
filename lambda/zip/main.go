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
	fmt.Printf("Starting zip generation for project: %s\n", request.ProjectID)

	// Get project details
	projectResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(request.ProjectID)},
		},
	})
	if err != nil || projectResult.Item == nil {
		return fmt.Errorf("project not found: %s", request.ProjectID)
	}

	var project Project
	if err := dynamodbattribute.UnmarshalMap(projectResult.Item, &project); err != nil {
		return fmt.Errorf("failed to unmarshal project: %v", err)
	}

	// Query all images in this project
	images, err := getProjectImages(request.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get project images: %v", err)
	}

	if len(images) == 0 {
		fmt.Println("No images in project, nothing to zip")
		return nil
	}

	fmt.Printf("Found %d images to zip\n", len(images))

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
		if len(batches) == 1 {
			zipKey = fmt.Sprintf("project-zips/%s/%s_%s.zip", s3Prefix, sanitizedName, dateStr)
		} else {
			zipKey = fmt.Sprintf("project-zips/%s/%s_%s_part%d.zip", s3Prefix, sanitizedName, dateStr, i+1)
		}

		// Create zip file
		zipInfo, err := createAndUploadZip(ctx, batch, zipKey)
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

	input := &dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectID-index"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
	}

	err := ddbClient.QueryPages(input, func(page *dynamodb.QueryOutput, lastPage bool) bool {
		for _, item := range page.Items {
			var img ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &img); err == nil {
				images = append(images, img)
			}
		}
		return !lastPage
	})

	return images, err
}

func splitIntoBatches(images []ImageRecord) [][]ImageRecord {
	var batches [][]ImageRecord
	var currentBatch []ImageRecord
	var currentSize int64

	for _, img := range images {
		// If adding this image would exceed limit, start new batch
		if currentSize+img.FileSize > maxZipSize && len(currentBatch) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = nil
			currentSize = 0
		}

		currentBatch = append(currentBatch, img)
		currentSize += img.FileSize
	}

	// Add the last batch
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

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

func createAndUploadZip(ctx context.Context, images []ImageRecord, zipKey string) (*ZipFile, error) {
	// Create a temporary file for the zip
	tmpFile, err := os.CreateTemp("", "project-*.zip")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Create zip writer
	zipWriter := zip.NewWriter(tmpFile)

	// Track file names to avoid duplicates
	fileNames := make(map[string]int)

	for _, img := range images {
		// Get unique filename
		baseName := filepath.Base(img.OriginalFile)
		fileName := baseName
		if count, exists := fileNames[baseName]; exists {
			ext := filepath.Ext(baseName)
			name := strings.TrimSuffix(baseName, ext)
			fileName = fmt.Sprintf("%s_%d%s", name, count+1, ext)
			fileNames[baseName] = count + 1
		} else {
			fileNames[baseName] = 1
		}

		// Download file from S3
		getResult, err := s3Client.GetObjectWithContext(ctx, &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(img.OriginalFile),
		})
		if err != nil {
			fmt.Printf("Warning: failed to download %s: %v\n", img.OriginalFile, err)
			continue
		}

		// Add file to zip
		writer, err := zipWriter.Create(fileName)
		if err != nil {
			getResult.Body.Close()
			fmt.Printf("Warning: failed to create zip entry for %s: %v\n", fileName, err)
			continue
		}

		_, err = io.Copy(writer, getResult.Body)
		getResult.Body.Close()
		if err != nil {
			fmt.Printf("Warning: failed to write %s to zip: %v\n", fileName, err)
		}
	}

	// Close zip writer
	if err := zipWriter.Close(); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to close zip writer: %v", err)
	}
	tmpFile.Close()

	// Get zip file size
	stat, err := os.Stat(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat zip file: %v", err)
	}

	// Upload to S3
	uploadFile, err := os.Open(tmpPath)
	if err != nil {
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
		return nil, fmt.Errorf("failed to upload zip: %v", err)
	}

	return &ZipFile{
		Key:        zipKey,
		Size:       stat.Size(),
		ImageCount: len(images),
		CreatedAt:  time.Now().Format(time.RFC3339),
		Status:     "complete",
	}, nil
}

func updateProjectZipFiles(projectID string, zipFiles []ZipFile) error {
	// Marshal zip files to DynamoDB format
	zipFilesList, err := dynamodbattribute.MarshalList(zipFiles)
	if err != nil {
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

	return err
}

func main() {
	lambda.Start(handleRequest)
}
