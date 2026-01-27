// fix_thumbnail_paths.go - Fixes thumbnail paths for recovered project images
//
// This script updates DynamoDB records that have NULL thumbnail paths
// by deriving the correct paths from the OriginalFile path.
//
// Usage:
//   go run fix_thumbnail_paths.go                    # Dry run
//   go run fix_thumbnail_paths.go -apply             # Apply fixes

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	defaultBucket     = "kill-snap"
	defaultImageTable = "kill-snap-ImageMetadata"
	awsRegion         = "us-east-2"
)

// ImageRecord represents a DynamoDB image record
type ImageRecord struct {
	ImageGUID    string `dynamodbav:"ImageGUID"`
	OriginalFile string `dynamodbav:"OriginalFile"`
	Thumbnail50  string `dynamodbav:"Thumbnail50"`
	Thumbnail400 string `dynamodbav:"Thumbnail400"`
	RawFile      string `dynamodbav:"RawFile"`
	Status       string `dynamodbav:"Status"`
}

func main() {
	apply := flag.Bool("apply", false, "Apply the fixes (default is dry-run)")
	verbose := flag.Bool("verbose", false, "Show detailed output")
	flag.Parse()

	bucketName := getEnvOrDefault("S3_BUCKET", defaultBucket)
	imageTable := getEnvOrDefault("IMAGE_TABLE", defaultImageTable)

	fmt.Printf("Image Table: %s\n", imageTable)
	fmt.Printf("S3 Bucket: %s\n", bucketName)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)
	s3Client := s3.New(sess)

	// Scan for project images with missing thumbnails
	fmt.Println("Scanning for project images with missing thumbnail paths...")

	var needsFix []ImageRecord
	var lastKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:        aws.String(imageTable),
			FilterExpression: aws.String("#status = :project AND begins_with(OriginalFile, :prefix)"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":project": {S: aws.String("project")},
				":prefix":  {S: aws.String("projects/")},
			},
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := ddbClient.Scan(input)
		if err != nil {
			fmt.Printf("Error scanning: %v\n", err)
			os.Exit(1)
		}

		for _, item := range result.Items {
			var record ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &record); err != nil {
				continue
			}

			// Check if thumbnail paths are missing or null
			if record.Thumbnail50 == "" || record.Thumbnail400 == "" {
				needsFix = append(needsFix, record)
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("Found %d records needing thumbnail path fixes\n\n", len(needsFix))

	if len(needsFix) == 0 {
		fmt.Println("No fixes needed!")
		return
	}

	// Fix each record
	fixed := 0
	errors := 0

	for _, record := range needsFix {
		// Derive thumbnail paths from original file
		dir := filepath.Dir(record.OriginalFile)
		baseName := strings.TrimSuffix(filepath.Base(record.OriginalFile), filepath.Ext(record.OriginalFile))
		ext := strings.ToLower(filepath.Ext(record.OriginalFile))

		thumb50 := fmt.Sprintf("%s/%s.50%s", dir, baseName, ext)
		thumb400 := fmt.Sprintf("%s/%s.400%s", dir, baseName, ext)

		// Check if thumbnails exist in S3
		thumb50Exists := checkS3Exists(s3Client, bucketName, thumb50)
		thumb400Exists := checkS3Exists(s3Client, bucketName, thumb400)

		// Also check for RAW file
		rawFile := findRawFile(s3Client, bucketName, dir, baseName)

		if *verbose {
			fmt.Printf("  %s:\n", record.ImageGUID)
			fmt.Printf("    Thumb50:  %s (exists: %v)\n", thumb50, thumb50Exists)
			fmt.Printf("    Thumb400: %s (exists: %v)\n", thumb400, thumb400Exists)
			if rawFile != "" {
				fmt.Printf("    RawFile:  %s\n", rawFile)
			}
		}

		if !*apply {
			fixed++
			continue
		}

		// Build update expression
		updateExpr := "SET "
		exprValues := make(map[string]*dynamodb.AttributeValue)
		parts := []string{}

		if thumb50Exists {
			parts = append(parts, "Thumbnail50 = :t50")
			exprValues[":t50"] = &dynamodb.AttributeValue{S: aws.String(thumb50)}
		}
		if thumb400Exists {
			parts = append(parts, "Thumbnail400 = :t400")
			exprValues[":t400"] = &dynamodb.AttributeValue{S: aws.String(thumb400)}
		}
		if rawFile != "" && record.RawFile == "" {
			parts = append(parts, "RawFile = :raw")
			exprValues[":raw"] = &dynamodb.AttributeValue{S: aws.String(rawFile)}
		}

		if len(parts) == 0 {
			continue
		}

		updateExpr += strings.Join(parts, ", ")

		_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(record.ImageGUID)},
			},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeValues: exprValues,
		})

		if err != nil {
			fmt.Printf("  ERROR updating %s: %v\n", record.ImageGUID, err)
			errors++
		} else {
			fixed++
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Records needing fixes: %d\n", len(needsFix))
	if *apply {
		fmt.Printf("Successfully fixed:    %d\n", fixed)
		if errors > 0 {
			fmt.Printf("Errors:                %d\n", errors)
		}
	} else {
		fmt.Printf("Would fix:             %d\n", fixed)
		fmt.Println("\nThis was a DRY RUN. To apply fixes, run:")
		fmt.Println("  go run fix_thumbnail_paths.go -apply")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func checkS3Exists(s3Client *s3.S3, bucket, key string) bool {
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

func findRawFile(s3Client *s3.S3, bucket, dir, baseName string) string {
	rawExtensions := []string{".raf", ".RAF", ".cr2", ".CR2", ".nef", ".NEF", ".arw", ".ARW", ".dng", ".DNG"}

	for _, ext := range rawExtensions {
		key := fmt.Sprintf("%s/%s%s", dir, baseName, ext)
		if checkS3Exists(s3Client, bucket, key) {
			return key
		}
	}
	return ""
}
