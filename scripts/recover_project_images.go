// recover_project_images.go - Rebuilds DynamoDB records for project images
//
// This script scans S3 project folders for original images and creates
// DynamoDB records for any that are missing.
//
// Usage:
//   go run recover_project_images.go                    # Dry run - show what would be created
//   go run recover_project_images.go -apply             # Apply the changes
//   go run recover_project_images.go -apply -verbose    # Apply with detailed output

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	defaultBucket       = "kill-snap"
	defaultRegion       = "us-east-2" // S3 bucket is in us-east-2
	defaultImageTable   = "kill-snap-ImageMetadata"
	defaultProjectTable = "kill-snap-Projects"
	ddbRegion           = "us-east-2" // DynamoDB is in us-east-2
	projectPrefix       = "projects/"
)

var (
	bucketName   string
	imageTable   string
	projectTable string
)

func init() {
	bucketName = getEnvOrDefault("S3_BUCKET", defaultBucket)
	imageTable = getEnvOrDefault("IMAGE_TABLE", defaultImageTable)
	projectTable = getEnvOrDefault("PROJECT_TABLE", defaultProjectTable)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ImageMetadata represents a DynamoDB image record
type ImageMetadata struct {
	ImageGUID        string            `dynamodbav:"ImageGUID"`
	OriginalFile     string            `dynamodbav:"OriginalFile"`
	OriginalFilename string            `dynamodbav:"OriginalFilename"`
	RawFile          string            `dynamodbav:"RawFile,omitempty"`
	Bucket           string            `dynamodbav:"Bucket"`
	Thumbnail50      string            `dynamodbav:"Thumbnail50"`
	Thumbnail400     string            `dynamodbav:"Thumbnail400"`
	Status           string            `dynamodbav:"Status"`
	ProjectID        string            `dynamodbav:"ProjectID"`
	Reviewed         string            `dynamodbav:"Reviewed"`
	FileSize         int64             `dynamodbav:"FileSize"`
	InsertedDateTime string            `dynamodbav:"InsertedDateTime"`
	UpdatedDateTime  string            `dynamodbav:"UpdatedDateTime"`
}

// Project represents a project record
type Project struct {
	ProjectID  string `dynamodbav:"ProjectID"`
	Name       string `dynamodbav:"Name"`
	S3Prefix   string `dynamodbav:"S3Prefix"`
	ImageCount int    `dynamodbav:"ImageCount"`
}

// S3ImageInfo holds information about an image in S3
type S3ImageInfo struct {
	Key           string
	Size          int64
	GUID          string
	OriginalName  string
	ProjectPrefix string
	Thumbnail50   string
	Thumbnail400  string
	RawFile       string
}

// Stats tracks recovery statistics
type Stats struct {
	TotalImagesFound    int
	AlreadyHaveRecords  int
	NeedRecords         int
	RecordsCreated      int
	Errors              int
	ByProject           map[string]int
	ProjectsUpdated     map[string]int
}

func main() {
	apply := flag.Bool("apply", false, "Apply the changes (default is dry-run)")
	verbose := flag.Bool("verbose", false, "Show detailed output")
	flag.Parse()

	fmt.Printf("S3 Bucket: %s (region: %s)\n", bucketName, defaultRegion)
	fmt.Printf("Image Table: %s (region: %s)\n", imageTable, ddbRegion)
	fmt.Printf("Project Table: %s (region: %s)\n", projectTable, ddbRegion)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	// Create AWS sessions for different regions
	s3Sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(defaultRegion),
	}))
	ddbSess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(ddbRegion),
	}))

	s3Client := s3.New(s3Sess)
	ddbClient := dynamodb.New(ddbSess)

	stats := Stats{
		ByProject:       make(map[string]int),
		ProjectsUpdated: make(map[string]int),
	}

	// Load projects to get S3Prefix -> ProjectID mapping
	fmt.Println("Loading projects...")
	projectMap, err := loadProjects(ddbClient)
	if err != nil {
		fmt.Printf("Error loading projects: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d projects\n\n", len(projectMap))

	// Scan S3 for original images in project folders
	fmt.Println("Scanning S3 for project images...")
	images, err := scanProjectImages(s3Client)
	if err != nil {
		fmt.Printf("Error scanning S3: %v\n", err)
		os.Exit(1)
	}
	stats.TotalImagesFound = len(images)
	fmt.Printf("Found %d original images in project folders\n\n", len(images))

	// Check which images need DynamoDB records
	fmt.Println("Checking for existing DynamoDB records...")
	needRecords := []S3ImageInfo{}

	for _, img := range images {
		exists, err := recordExists(ddbClient, img.GUID)
		if err != nil {
			fmt.Printf("  Error checking %s: %v\n", img.GUID, err)
			stats.Errors++
			continue
		}

		if exists {
			stats.AlreadyHaveRecords++
			if *verbose {
				fmt.Printf("  [EXISTS] %s\n", img.Key)
			}
		} else {
			stats.NeedRecords++
			stats.ByProject[img.ProjectPrefix]++
			needRecords = append(needRecords, img)
			if *verbose {
				fmt.Printf("  [MISSING] %s\n", img.Key)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Images with existing records: %d\n", stats.AlreadyHaveRecords)
	fmt.Printf("Images needing records:       %d\n", stats.NeedRecords)
	fmt.Println()

	if stats.NeedRecords > 0 {
		fmt.Println("By project:")
		for project, count := range stats.ByProject {
			fmt.Printf("  %s: %d images\n", project, count)
		}
		fmt.Println()
	}

	if !*apply {
		if stats.NeedRecords > 0 {
			fmt.Println("This was a DRY RUN. To create these records, run:")
			fmt.Println("  go run recover_project_images.go -apply")
		} else {
			fmt.Println("No records need to be created.")
		}
		return
	}

	// Create DynamoDB records
	fmt.Println("Creating DynamoDB records...")
	for _, img := range needRecords {
		projectID := projectMap[img.ProjectPrefix]
		if projectID == "" {
			fmt.Printf("  WARNING: No project found for prefix '%s', skipping %s\n", img.ProjectPrefix, img.Key)
			stats.Errors++
			continue
		}

		record := ImageMetadata{
			ImageGUID:        img.GUID,
			OriginalFile:     img.Key,
			OriginalFilename: img.OriginalName,
			RawFile:          img.RawFile,
			Bucket:           bucketName,
			Thumbnail50:      img.Thumbnail50,
			Thumbnail400:     img.Thumbnail400,
			Status:           "project",
			ProjectID:        projectID,
			Reviewed:         "true",
			FileSize:         img.Size,
			InsertedDateTime: time.Now().Format(time.RFC3339),
			UpdatedDateTime:  time.Now().Format(time.RFC3339),
		}

		if *verbose {
			fmt.Printf("  Creating record for %s (project: %s)\n", img.GUID, img.ProjectPrefix)
		}

		err := createRecord(ddbClient, record)
		if err != nil {
			fmt.Printf("  ERROR creating record for %s: %v\n", img.GUID, err)
			stats.Errors++
		} else {
			stats.RecordsCreated++
			stats.ProjectsUpdated[img.ProjectPrefix]++
		}
	}

	// Update project image counts
	fmt.Println("\nUpdating project image counts...")
	for projectPrefix, addedCount := range stats.ProjectsUpdated {
		projectID := projectMap[projectPrefix]
		if projectID == "" {
			continue
		}

		// Get current count
		currentCount, err := getProjectImageCount(ddbClient, projectID)
		if err != nil {
			fmt.Printf("  Error getting count for %s: %v\n", projectPrefix, err)
			continue
		}

		newCount := currentCount + addedCount
		err = updateProjectImageCount(ddbClient, projectID, newCount)
		if err != nil {
			fmt.Printf("  Error updating count for %s: %v\n", projectPrefix, err)
		} else {
			fmt.Printf("  %s: %d -> %d (+%d)\n", projectPrefix, currentCount, newCount, addedCount)
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total images found:         %d\n", stats.TotalImagesFound)
	fmt.Printf("Already had records:        %d\n", stats.AlreadyHaveRecords)
	fmt.Printf("Records created:            %d\n", stats.RecordsCreated)
	if stats.Errors > 0 {
		fmt.Printf("Errors:                     %d\n", stats.Errors)
	}
}

// loadProjects loads all projects and returns a map of S3Prefix -> ProjectID
func loadProjects(ddbClient *dynamodb.DynamoDB) (map[string]string, error) {
	projectMap := make(map[string]string)

	input := &dynamodb.ScanInput{
		TableName: aws.String(projectTable),
	}

	result, err := ddbClient.Scan(input)
	if err != nil {
		return nil, err
	}

	for _, item := range result.Items {
		var project Project
		if err := dynamodbattribute.UnmarshalMap(item, &project); err != nil {
			continue
		}
		if project.S3Prefix != "" {
			projectMap[project.S3Prefix] = project.ProjectID
		}
	}

	return projectMap, nil
}

// scanProjectImages scans S3 for original images in project folders
func scanProjectImages(s3Client *s3.S3) ([]S3ImageInfo, error) {
	images := []S3ImageInfo{}
	allFiles := make(map[string]*s3.Object) // key -> object

	// First pass: collect all files
	var continuationToken *string
	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
			Prefix: aws.String(projectPrefix),
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		result, err := s3Client.ListObjectsV2(input)
		if err != nil {
			return nil, err
		}

		for _, obj := range result.Contents {
			key := *obj.Key
			allFiles[key] = obj
		}

		if result.IsTruncated == nil || !*result.IsTruncated {
			break
		}
		continuationToken = result.NextContinuationToken
	}

	// Second pass: identify original images and their related files
	for key, obj := range allFiles {
		// Skip thumbnails, zips, and non-images
		if isThumbOrZip(key) {
			continue
		}

		// Skip RAW files (they'll be associated with their JPG)
		if isRawFile(key) {
			continue
		}

		// This is an original image
		guid := extractGUID(key)
		if guid == "" {
			// Use filename as identifier for non-UUID files
			guid = strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))
		}

		// Extract project prefix (first folder after projects/)
		projectPrefix := extractProjectPrefix(key)

		// Find related files
		dir := filepath.Dir(key)
		baseName := strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))

		thumb50 := findRelatedFile(allFiles, dir, baseName, ".50.")
		thumb400 := findRelatedFile(allFiles, dir, baseName, ".400.")
		rawFile := findRawFile(allFiles, dir, baseName)

		img := S3ImageInfo{
			Key:           key,
			Size:          *obj.Size,
			GUID:          guid,
			OriginalName:  strings.TrimSuffix(filepath.Base(key), filepath.Ext(key)),
			ProjectPrefix: projectPrefix,
			Thumbnail50:   thumb50,
			Thumbnail400:  thumb400,
			RawFile:       rawFile,
		}
		images = append(images, img)
	}

	return images, nil
}

// isThumbOrZip checks if a file is a thumbnail or zip
func isThumbOrZip(key string) bool {
	lower := strings.ToLower(key)
	return strings.Contains(lower, ".50.") ||
		strings.Contains(lower, ".400.") ||
		strings.HasSuffix(lower, ".zip")
}

// isRawFile checks if a file is a RAW image
func isRawFile(key string) bool {
	lower := strings.ToLower(key)
	rawExtensions := []string{".raf", ".cr2", ".cr3", ".nef", ".arw", ".dng", ".orf", ".rw2", ".pef", ".srw"}
	for _, ext := range rawExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// extractGUID extracts UUID from filename if present
func extractGUID(key string) string {
	fileName := filepath.Base(key)
	baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

	// Check if it looks like a UUID (8-4-4-4-12 pattern)
	if len(baseName) == 36 && strings.Count(baseName, "-") == 4 {
		return baseName
	}
	return ""
}

// extractProjectPrefix extracts the project folder name from a key
func extractProjectPrefix(key string) string {
	// Key format: projects/{project_prefix}/...
	trimmed := strings.TrimPrefix(key, projectPrefix)
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// findRelatedFile finds a related file with a specific pattern
func findRelatedFile(files map[string]*s3.Object, dir, baseName, pattern string) string {
	// Try common patterns
	patterns := []string{
		dir + "/" + baseName + pattern + "jpg",
		dir + "/" + baseName + pattern + "JPG",
		dir + "/" + baseName + pattern + "jpeg",
		dir + "/" + baseName + pattern + "JPEG",
	}

	for _, p := range patterns {
		if _, exists := files[p]; exists {
			return p
		}
	}
	return ""
}

// findRawFile finds a RAW file with the same base name
func findRawFile(files map[string]*s3.Object, dir, baseName string) string {
	rawExtensions := []string{".raf", ".RAF", ".cr2", ".CR2", ".cr3", ".CR3", ".nef", ".NEF", ".arw", ".ARW", ".dng", ".DNG"}

	for _, ext := range rawExtensions {
		key := dir + "/" + baseName + ext
		if _, exists := files[key]; exists {
			return key
		}
	}
	return ""
}

// recordExists checks if a DynamoDB record exists for the given GUID
func recordExists(ddbClient *dynamodb.DynamoDB, guid string) (bool, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(guid)},
		},
		ProjectionExpression: aws.String("ImageGUID"),
	}

	result, err := ddbClient.GetItem(input)
	if err != nil {
		return false, err
	}

	return result.Item != nil, nil
}

// createRecord creates a new DynamoDB record
func createRecord(ddbClient *dynamodb.DynamoDB, record ImageMetadata) error {
	av, err := dynamodbattribute.MarshalMap(record)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(imageTable),
		Item:      av,
	}

	_, err = ddbClient.PutItem(input)
	return err
}

// getProjectImageCount gets the current image count for a project
func getProjectImageCount(ddbClient *dynamodb.DynamoDB, projectID string) (int, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(projectTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		ProjectionExpression: aws.String("ImageCount"),
	}

	result, err := ddbClient.GetItem(input)
	if err != nil {
		return 0, err
	}

	if result.Item == nil {
		return 0, nil
	}

	var project Project
	if err := dynamodbattribute.UnmarshalMap(result.Item, &project); err != nil {
		return 0, err
	}

	return project.ImageCount, nil
}

// updateProjectImageCount updates the image count for a project
func updateProjectImageCount(ddbClient *dynamodb.DynamoDB, projectID string, count int) error {
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(projectTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ImageCount = :count"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":count": {N: aws.String(fmt.Sprintf("%d", count))},
		},
	}

	_, err := ddbClient.UpdateItem(input)
	return err
}
