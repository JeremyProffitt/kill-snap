// cleanup_test_folders.go - Removes orphan test folders from S3
//
// This script identifies S3 project folders that don't have corresponding
// entries in the Projects DynamoDB table and removes them.
//
// Usage:
//   go run cleanup_test_folders.go                    # Dry run - show what would be deleted
//   go run cleanup_test_folders.go -apply             # Apply deletions

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	defaultBucket       = "kill-snap"
	defaultProjectTable = "kill-snap-Projects"
	awsRegion           = "us-east-2"
	projectPrefix       = "projects/"
)

type Project struct {
	ProjectID string `dynamodbav:"ProjectID"`
	S3Prefix  string `dynamodbav:"S3Prefix"`
}

func main() {
	apply := flag.Bool("apply", false, "Apply the deletions (default is dry-run)")
	flag.Parse()

	bucketName := getEnvOrDefault("S3_BUCKET", defaultBucket)
	projectTable := getEnvOrDefault("PROJECT_TABLE", defaultProjectTable)

	fmt.Printf("S3 Bucket: %s\n", bucketName)
	fmt.Printf("Project Table: %s\n", projectTable)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)
	s3Client := s3.New(sess)

	// Get valid project prefixes from DynamoDB
	fmt.Println("Fetching valid project prefixes...")
	validPrefixes := make(map[string]bool)

	var lastKey map[string]*dynamodb.AttributeValue
	for {
		input := &dynamodb.ScanInput{
			TableName:            aws.String(projectTable),
			ProjectionExpression: aws.String("S3Prefix"),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := ddbClient.Scan(input)
		if err != nil {
			fmt.Printf("Error scanning projects: %v\n", err)
			os.Exit(1)
		}

		for _, item := range result.Items {
			var project Project
			if err := dynamodbattribute.UnmarshalMap(item, &project); err != nil {
				continue
			}
			if project.S3Prefix != "" {
				validPrefixes[project.S3Prefix] = true
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}
	fmt.Printf("Found %d valid project prefixes\n\n", len(validPrefixes))

	// List all top-level folders in projects/
	fmt.Println("Listing S3 project folders...")
	s3Folders := []string{}

	listInput := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucketName),
		Prefix:    aws.String(projectPrefix),
		Delimiter: aws.String("/"),
	}

	err := s3Client.ListObjectsV2Pages(listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, prefix := range page.CommonPrefixes {
			folder := strings.TrimPrefix(*prefix.Prefix, projectPrefix)
			folder = strings.TrimSuffix(folder, "/")
			s3Folders = append(s3Folders, folder)
		}
		return true
	})
	if err != nil {
		fmt.Printf("Error listing S3: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d S3 folders\n\n", len(s3Folders))

	// Identify orphan folders
	orphanFolders := []string{}
	for _, folder := range s3Folders {
		if !validPrefixes[folder] {
			orphanFolders = append(orphanFolders, folder)
		}
	}

	if len(orphanFolders) == 0 {
		fmt.Println("No orphan folders found!")
		return
	}

	fmt.Printf("Found %d orphan folders:\n", len(orphanFolders))
	totalFiles := 0
	folderFileCounts := make(map[string]int)

	for _, folder := range orphanFolders {
		// Count files in folder
		count := 0
		prefix := projectPrefix + folder + "/"
		err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
			Prefix: aws.String(prefix),
		}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			count += len(page.Contents)
			return true
		})
		if err != nil {
			fmt.Printf("  %s: ERROR counting files\n", folder)
			continue
		}
		folderFileCounts[folder] = count
		totalFiles += count
		fmt.Printf("  %s: %d files\n", folder, count)
	}

	fmt.Printf("\nTotal files to delete: %d\n\n", totalFiles)

	if !*apply {
		fmt.Println("This was a DRY RUN. To delete these folders, run:")
		fmt.Println("  go run cleanup_test_folders.go -apply")
		return
	}

	// Delete orphan folders
	fmt.Println("Deleting orphan folders...")
	deletedFiles := 0
	errors := 0

	for _, folder := range orphanFolders {
		prefix := projectPrefix + folder + "/"
		fmt.Printf("  Deleting %s...\n", folder)

		// List and delete all objects in the folder
		err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
			Bucket: aws.String(bucketName),
			Prefix: aws.String(prefix),
		}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			if len(page.Contents) == 0 {
				return true
			}

			// Build delete request
			objects := make([]*s3.ObjectIdentifier, len(page.Contents))
			for i, obj := range page.Contents {
				objects[i] = &s3.ObjectIdentifier{Key: obj.Key}
			}

			_, err := s3Client.DeleteObjects(&s3.DeleteObjectsInput{
				Bucket: aws.String(bucketName),
				Delete: &s3.Delete{
					Objects: objects,
					Quiet:   aws.Bool(true),
				},
			})
			if err != nil {
				fmt.Printf("    ERROR: %v\n", err)
				errors++
			} else {
				deletedFiles += len(objects)
			}
			return true
		})
		if err != nil {
			fmt.Printf("    ERROR listing: %v\n", err)
			errors++
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Orphan folders found: %d\n", len(orphanFolders))
	fmt.Printf("Files deleted: %d\n", deletedFiles)
	if errors > 0 {
		fmt.Printf("Errors: %d\n", errors)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
