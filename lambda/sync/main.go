package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
)

var (
	ddbClient  *dynamodb.DynamoDB
	s3Client   *s3.S3
	imageTable string
	bucketName string
)

// ImageRecord represents a record in the ImageMetadata table
type ImageRecord struct {
	ImageGUID    string `json:"imageGUID" dynamodbav:"ImageGUID"`
	OriginalFile string `json:"originalFile" dynamodbav:"OriginalFile"`
	Thumbnail50  string `json:"thumbnail50" dynamodbav:"Thumbnail50"`
	Thumbnail400 string `json:"thumbnail400" dynamodbav:"Thumbnail400"`
	Bucket       string `json:"bucket" dynamodbav:"Bucket"`
	Status       string `json:"status" dynamodbav:"Status"`
}

// SyncResult contains the results of the sync operation
type SyncResult struct {
	TotalScanned   int      `json:"totalScanned"`
	OrphansRemoved int      `json:"orphansRemoved"`
	OrphanIDs      []string `json:"orphanIds,omitempty"`
	Errors         []string `json:"errors,omitempty"`
	Duration       string   `json:"duration"`
}

func init() {
	sess := session.Must(session.NewSession())
	ddbClient = dynamodb.New(sess)
	s3Client = s3.New(sess)

	imageTable = os.Getenv("IMAGE_TABLE")
	bucketName = os.Getenv("BUCKET_NAME")

	if imageTable == "" {
		imageTable = "kill-snap-ImageMetadata"
	}
	if bucketName == "" {
		bucketName = "kill-snap-images"
	}
}

// checkS3ObjectExists checks if an object exists in S3
func checkS3ObjectExists(bucket, key string) bool {
	if key == "" {
		return false
	}

	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})

	return err == nil
}

// deleteFromDynamoDB deletes a record from DynamoDB
func deleteFromDynamoDB(imageGUID string) error {
	_, err := ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
	})
	return err
}

// handler is the Lambda function handler
func handler(ctx context.Context) (SyncResult, error) {
	startTime := time.Now()
	result := SyncResult{
		OrphanIDs: []string{},
		Errors:    []string{},
	}

	fmt.Printf("Starting DynamoDB-S3 sync at %s\n", startTime.Format(time.RFC3339))
	fmt.Printf("Image table: %s, Bucket: %s\n", imageTable, bucketName)

	// Scan all items from DynamoDB
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue
	for {
		scanInput := &dynamodb.ScanInput{
			TableName: aws.String(imageTable),
			ProjectionExpression: aws.String("ImageGUID, OriginalFile, #bucket, #status"),
			ExpressionAttributeNames: map[string]*string{
				"#bucket": aws.String("Bucket"),
				"#status": aws.String("Status"),
			},
		}

		if lastEvaluatedKey != nil {
			scanInput.ExclusiveStartKey = lastEvaluatedKey
		}

		scanOutput, err := ddbClient.Scan(scanInput)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to scan DynamoDB: %v", err)
			fmt.Println(errMsg)
			result.Errors = append(result.Errors, errMsg)
			break
		}

		// Process each item
		for _, item := range scanOutput.Items {
			result.TotalScanned++

			var record ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &record); err != nil {
				errMsg := fmt.Sprintf("Failed to unmarshal item: %v", err)
				fmt.Println(errMsg)
				result.Errors = append(result.Errors, errMsg)
				continue
			}

			// Skip project images - they have different path conventions and should not be
			// considered orphans even if the file has moved within the project folder
			if record.Status == "project" || strings.HasPrefix(record.OriginalFile, "projects/") {
				continue
			}

			// Use the bucket from the record if available, otherwise use env var
			bucket := record.Bucket
			if bucket == "" {
				bucket = bucketName
			}

			// Check if the original file exists in S3
			if !checkS3ObjectExists(bucket, record.OriginalFile) {
				fmt.Printf("Orphan found: %s (file: %s)\n", record.ImageGUID, record.OriginalFile)

				// Delete from DynamoDB
				if err := deleteFromDynamoDB(record.ImageGUID); err != nil {
					errMsg := fmt.Sprintf("Failed to delete orphan %s: %v", record.ImageGUID, err)
					fmt.Println(errMsg)
					result.Errors = append(result.Errors, errMsg)
				} else {
					result.OrphansRemoved++
					result.OrphanIDs = append(result.OrphanIDs, record.ImageGUID)
					fmt.Printf("Removed orphan: %s\n", record.ImageGUID)
				}
			}

			// Log progress every 100 items
			if result.TotalScanned%100 == 0 {
				fmt.Printf("Progress: scanned %d items, found %d orphans\n", result.TotalScanned, result.OrphansRemoved)
			}
		}

		// Check if there are more items to scan
		lastEvaluatedKey = scanOutput.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	result.Duration = time.Since(startTime).String()

	fmt.Printf("\nSync completed in %s\n", result.Duration)
	fmt.Printf("Total scanned: %d\n", result.TotalScanned)
	fmt.Printf("Orphans removed: %d\n", result.OrphansRemoved)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}

	return result, nil
}

func main() {
	lambda.Start(handler)
}
