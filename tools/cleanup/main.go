package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	s3Bucket          = "kill-snap"
	imageTable        = "kill-snap-ImageMetadata"
	projectsTable     = "kill-snap-Projects"
	reviewGroupsTable = "kill-snap-ReviewGroups"
	usersTable        = "kill-snap-Users" // PRESERVED - not deleted
)

func main() {
	fmt.Println()
	fmt.Println("============================================")
	fmt.Println(" KILL-SNAP DATA CLEANUP TOOL")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Println("This tool will DELETE ALL DATA from:")
	fmt.Printf("  - DynamoDB Table: %s\n", imageTable)
	fmt.Printf("  - DynamoDB Table: %s\n", projectsTable)
	fmt.Printf("  - DynamoDB Table: %s\n", reviewGroupsTable)
	fmt.Printf("  - S3 Bucket: %s\n", s3Bucket)
	fmt.Println()
	fmt.Println("This tool will PRESERVE (not delete):")
	fmt.Printf("  - DynamoDB Table: %s (user accounts)\n", usersTable)
	fmt.Println()
	fmt.Println("============================================")
	fmt.Println()

	// Confirmation prompt
	fmt.Print("Are you sure you want to delete ALL data? (yes/no): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "yes" {
		fmt.Println()
		fmt.Println("Operation cancelled.")
		return
	}

	fmt.Println()
	fmt.Println("Starting cleanup...")
	fmt.Println()

	// Initialize AWS session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-2"),
	})
	if err != nil {
		fmt.Printf("Failed to create AWS session: %v\n", err)
		os.Exit(1)
	}

	ddbClient := dynamodb.New(sess)
	s3Client := s3.New(sess)

	// Step 1: Delete all items from ImageMetadata table
	fmt.Printf("[1/4] Deleting all items from %s...\n", imageTable)
	deleted, err := deleteAllFromTable(ddbClient, imageTable, "ImageGUID", "")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Deleted %d items from %s\n", deleted, imageTable)
	}
	fmt.Println()

	// Step 2: Delete all items from Projects table
	fmt.Printf("[2/4] Deleting all items from %s...\n", projectsTable)
	deleted, err = deleteAllFromTable(ddbClient, projectsTable, "ProjectID", "")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Deleted %d items from %s\n", deleted, projectsTable)
	}
	fmt.Println()

	// Step 3: Delete all items from ReviewGroups table (composite key)
	fmt.Printf("[3/4] Deleting all items from %s...\n", reviewGroupsTable)
	deleted, err = deleteAllFromTable(ddbClient, reviewGroupsTable, "ReviewID", "ImageGUID")
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Deleted %d items from %s\n", deleted, reviewGroupsTable)
	}
	fmt.Println()

	// Step 4: Delete all objects from S3 bucket
	fmt.Printf("[4/4] Deleting all objects from S3 bucket: %s...\n", s3Bucket)
	deletedObjects, err := deleteAllFromS3(s3Client, s3Bucket)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		fmt.Printf("  Deleted %d objects from s3://%s\n", deletedObjects, s3Bucket)
	}
	fmt.Println()

	// Completion
	fmt.Println("============================================")
	fmt.Println(" CLEANUP COMPLETE")
	fmt.Println("============================================")
	fmt.Println()
	fmt.Println("All data has been deleted from:")
	fmt.Printf("  - %s\n", imageTable)
	fmt.Printf("  - %s\n", projectsTable)
	fmt.Printf("  - %s\n", reviewGroupsTable)
	fmt.Printf("  - s3://%s\n", s3Bucket)
	fmt.Println()
	fmt.Println("Preserved (not deleted):")
	fmt.Printf("  - %s (user accounts)\n", usersTable)
	fmt.Println()
}

// deleteAllFromTable scans a DynamoDB table and deletes all items
// hashKey is the partition key name, rangeKey is the sort key name (empty string if none)
func deleteAllFromTable(client *dynamodb.DynamoDB, tableName, hashKey, rangeKey string) (int, error) {
	totalDeleted := 0

	// Build projection expression
	projectionExpr := hashKey
	if rangeKey != "" {
		projectionExpr += ", " + rangeKey
	}

	// Scan with pagination
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:            aws.String(tableName),
			ProjectionExpression: aws.String(projectionExpr),
		}

		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := client.Scan(input)
		if err != nil {
			return totalDeleted, fmt.Errorf("scan failed: %w", err)
		}

		if len(result.Items) == 0 {
			break
		}

		fmt.Printf("  Found %d items to delete...\n", len(result.Items))

		// Delete each item
		for _, item := range result.Items {
			key := map[string]*dynamodb.AttributeValue{
				hashKey: item[hashKey],
			}
			if rangeKey != "" {
				key[rangeKey] = item[rangeKey]
			}

			// Get key values for logging
			hashVal := ""
			if item[hashKey] != nil && item[hashKey].S != nil {
				hashVal = *item[hashKey].S
			}

			_, err := client.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(tableName),
				Key:       key,
			})
			if err != nil {
				fmt.Printf("  Warning: failed to delete item %s: %v\n", hashVal, err)
				continue
			}

			totalDeleted++
			fmt.Printf("  Deleted: %s\n", hashVal)
		}

		// Check if there are more items
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	return totalDeleted, nil
}

// deleteAllFromS3 deletes all objects from an S3 bucket
func deleteAllFromS3(client *s3.S3, bucket string) (int, error) {
	// Count and collect objects first
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	}

	var objects []*s3.Object
	err := client.ListObjectsV2Pages(listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		objects = append(objects, page.Contents...)
		return true
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		fmt.Println("  No objects found in bucket")
		return 0, nil
	}

	fmt.Printf("  Found %d objects to delete...\n", len(objects))

	// Log each object being deleted (verbose logging)
	for _, obj := range objects {
		if obj.Key != nil {
			fmt.Printf("  Deleting: %s\n", *obj.Key)
		}
	}

	// Use batch delete iterator for efficiency
	iter := s3manager.NewDeleteListIterator(client, &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
	})

	batcher := s3manager.NewBatchDeleteWithClient(client)

	// Delete all objects
	if err := batcher.Delete(aws.BackgroundContext(), iter); err != nil {
		return 0, fmt.Errorf("failed to delete objects: %w", err)
	}

	return len(objects), nil
}
