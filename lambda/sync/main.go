package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
)

const (
	// maxDeletionsPerRun prevents mass deletions due to bugs or misconfigurations.
	// If this threshold is reached, the sync will abort and require manual review.
	maxDeletionsPerRun = 100
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
	TotalScanned      int      `json:"totalScanned"`
	OrphansRemoved    int      `json:"orphansRemoved"`
	OrphansRepaired   int      `json:"orphansRepaired"`
	ThumbnailsDeleted int      `json:"thumbnailsDeleted"`
	OrphanIDs         []string `json:"orphanIds,omitempty"`
	RepairedIDs       []string `json:"repairedIds,omitempty"`
	Errors            []string `json:"errors,omitempty"`
	Duration          string   `json:"duration"`
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

// projectFolderPrefix extracts the top-level project folder ("projects/{name}/")
// from an object key, or "" if the key is not under projects/.
func projectFolderPrefix(key string) string {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) < 3 || parts[0] != "projects" || parts[1] == "" {
		return ""
	}
	return parts[0] + "/" + parts[1] + "/"
}

// listProjectFolder lists all object keys under a project folder, caching per run.
func listProjectFolder(bucket, prefix string, cache map[string][]string) ([]string, error) {
	cacheKey := bucket + "|" + prefix
	if keys, ok := cache[cacheKey]; ok {
		return keys, nil
	}

	var keys []string
	err := s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}

	cache[cacheKey] = keys
	return keys, nil
}

// findProjectFileByGUID searches a project folder for the record's original file
// (matched by GUID + extension, excluding .50./.400. thumbnails). Project files
// can legitimately move within their project folder, so a missing key may just
// mean the path in DynamoDB is stale rather than the file being gone.
func findProjectFileByGUID(bucket string, record ImageRecord, cache map[string][]string) (string, bool, error) {
	prefix := projectFolderPrefix(record.OriginalFile)
	if prefix == "" {
		return "", false, nil
	}

	keys, err := listProjectFolder(bucket, prefix, cache)
	if err != nil {
		return "", false, err
	}

	ext := filepath.Ext(record.OriginalFile)
	want := "/" + record.ImageGUID + ext
	for _, key := range keys {
		if strings.HasSuffix(key, want) && !strings.Contains(key, ".50.") && !strings.Contains(key, ".400.") {
			return key, true, nil
		}
	}
	return "", false, nil
}

// repairProjectRecord points a project record at the relocated original file
// (and its sibling thumbnails, which move together with the original).
func repairProjectRecord(imageGUID, newKey string) error {
	ext := filepath.Ext(newKey)
	base := strings.TrimSuffix(newKey, ext)

	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, UpdatedDateTime = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":    {S: aws.String(newKey)},
			":t50":     {S: aws.String(base + ".50" + ext)},
			":t400":    {S: aws.String(base + ".400" + ext)},
			":updated": {S: aws.String(time.Now().UTC().Format(time.RFC3339))},
		},
		ConditionExpression: aws.String("attribute_exists(ImageGUID)"),
	})
	return err
}

// deleteOrphanThumbnails removes the leftover thumbnail objects of a deleted
// orphan record so they don't accumulate as unreferenced junk in S3.
func deleteOrphanThumbnails(bucket string, record ImageRecord) int {
	deleted := 0
	for _, key := range []string{record.Thumbnail50, record.Thumbnail400} {
		if key == "" {
			continue
		}
		if _, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		}); err != nil {
			fmt.Printf("WARNING: failed to delete orphan thumbnail %s: %v\n", key, err)
		} else {
			deleted++
			fmt.Printf("Deleted orphan thumbnail: %s\n", key)
		}
	}
	return deleted
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
	aborted := false
	projectFolderCache := make(map[string][]string)

	for {
		// Safety check: abort if too many deletions to prevent mass data loss
		if result.OrphansRemoved >= maxDeletionsPerRun {
			errMsg := fmt.Sprintf("SAFETY ABORT: Deletion threshold reached (%d). Manual review required.", maxDeletionsPerRun)
			fmt.Println(errMsg)
			result.Errors = append(result.Errors, errMsg)
			aborted = true
			break
		}
		scanInput := &dynamodb.ScanInput{
			TableName: aws.String(imageTable),
			ProjectionExpression: aws.String("ImageGUID, OriginalFile, Thumbnail50, Thumbnail400, #bucket, #status"),
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

			// Use the bucket from the record if available, otherwise use env var
			bucket := record.Bucket
			if bucket == "" {
				bucket = bucketName
			}

			// Check if the original file exists in S3
			if !checkS3ObjectExists(bucket, record.OriginalFile) {
				// Project files can legitimately move within their project folder,
				// leaving the DynamoDB path stale. Search the folder by GUID and
				// repair the record before treating it as an orphan.
				if record.Status == "project" || strings.HasPrefix(record.OriginalFile, "projects/") {
					newKey, found, err := findProjectFileByGUID(bucket, record, projectFolderCache)
					if err != nil {
						errMsg := fmt.Sprintf("Failed to search project folder for %s: %v (skipping, will retry next run)", record.ImageGUID, err)
						fmt.Println(errMsg)
						result.Errors = append(result.Errors, errMsg)
						continue
					}
					if found {
						fmt.Printf("Stale path for %s: %s -> %s\n", record.ImageGUID, record.OriginalFile, newKey)
						if err := repairProjectRecord(record.ImageGUID, newKey); err != nil {
							errMsg := fmt.Sprintf("Failed to repair record %s: %v", record.ImageGUID, err)
							fmt.Println(errMsg)
							result.Errors = append(result.Errors, errMsg)
						} else {
							result.OrphansRepaired++
							result.RepairedIDs = append(result.RepairedIDs, record.ImageGUID)
							fmt.Printf("Repaired record: %s\n", record.ImageGUID)
						}
						continue
					}
				}

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
					result.ThumbnailsDeleted += deleteOrphanThumbnails(bucket, record)
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

	if aborted {
		fmt.Printf("\nSync ABORTED after %s\n", result.Duration)
	} else {
		fmt.Printf("\nSync completed in %s\n", result.Duration)
	}
	fmt.Printf("Total scanned: %d\n", result.TotalScanned)
	fmt.Printf("Orphans removed: %d\n", result.OrphansRemoved)
	fmt.Printf("Orphans repaired: %d\n", result.OrphansRepaired)
	fmt.Printf("Orphan thumbnails deleted: %d\n", result.ThumbnailsDeleted)
	if len(result.Errors) > 0 {
		fmt.Printf("Errors: %d\n", len(result.Errors))
	}

	return result, nil
}

func main() {
	lambda.Start(handler)
}
