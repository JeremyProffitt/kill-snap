// find_duplicates.go - Finds and removes duplicate DynamoDB records
//
// Duplicates are identified by having the same OriginalFile path.
// Keeps the record with GroupNumber set (if any), or the most recent.
//
// Usage:
//   go run find_duplicates.go                    # Dry run
//   go run find_duplicates.go -apply             # Apply deletions

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	defaultImageTable = "kill-snap-ImageMetadata"
	awsRegion         = "us-east-2"
)

type ImageRecord struct {
	ImageGUID       string `dynamodbav:"ImageGUID"`
	OriginalFile    string `dynamodbav:"OriginalFile"`
	Status          string `dynamodbav:"Status"`
	GroupNumber     int    `dynamodbav:"GroupNumber"`
	UpdatedDateTime string `dynamodbav:"UpdatedDateTime"`
	ProjectID       string `dynamodbav:"ProjectID"`
}

func main() {
	apply := flag.Bool("apply", false, "Apply deletions (default is dry-run)")
	flag.Parse()

	imageTable := getEnvOrDefault("IMAGE_TABLE", defaultImageTable)

	fmt.Printf("Image Table: %s\n", imageTable)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)

	// Scan all records
	fmt.Println("Scanning all records...")
	recordsByFile := make(map[string][]ImageRecord)
	var lastKey map[string]*dynamodb.AttributeValue
	totalRecords := 0

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(imageTable),
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
			totalRecords++
			if record.OriginalFile != "" {
				recordsByFile[record.OriginalFile] = append(recordsByFile[record.OriginalFile], record)
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("Total records: %d\n", totalRecords)
	fmt.Printf("Unique files: %d\n", len(recordsByFile))

	// Find duplicates
	var duplicatesToDelete []ImageRecord
	duplicateFiles := 0

	for file, records := range recordsByFile {
		if len(records) <= 1 {
			continue
		}

		duplicateFiles++
		fmt.Printf("\nDuplicate: %s (%d records)\n", file, len(records))

		// Sort records: prefer ones with GroupNumber > 0, then by Status=project, then by UpdatedDateTime
		sort.Slice(records, func(i, j int) bool {
			// Prefer records with GroupNumber set
			if records[i].GroupNumber > 0 && records[j].GroupNumber == 0 {
				return true
			}
			if records[j].GroupNumber > 0 && records[i].GroupNumber == 0 {
				return false
			}
			// Prefer project status
			if records[i].Status == "project" && records[j].Status != "project" {
				return true
			}
			if records[j].Status == "project" && records[i].Status != "project" {
				return false
			}
			// Prefer records with ProjectID
			if records[i].ProjectID != "" && records[j].ProjectID == "" {
				return true
			}
			if records[j].ProjectID != "" && records[i].ProjectID == "" {
				return false
			}
			// Fall back to UpdatedDateTime
			return records[i].UpdatedDateTime > records[j].UpdatedDateTime
		})

		// Keep the first (best) record, delete the rest
		keeper := records[0]
		fmt.Printf("  KEEP: %s (Status=%s, Group=%d, ProjectID=%s)\n",
			keeper.ImageGUID, keeper.Status, keeper.GroupNumber, keeper.ProjectID)

		for _, record := range records[1:] {
			fmt.Printf("  DELETE: %s (Status=%s, Group=%d)\n",
				record.ImageGUID, record.Status, record.GroupNumber)
			duplicatesToDelete = append(duplicatesToDelete, record)
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Files with duplicates: %d\n", duplicateFiles)
	fmt.Printf("Records to delete: %d\n", len(duplicatesToDelete))

	if len(duplicatesToDelete) == 0 {
		fmt.Println("No duplicates found!")
		return
	}

	if !*apply {
		fmt.Println("\nThis was a DRY RUN. To delete duplicates, run:")
		fmt.Println("  go run find_duplicates.go -apply")
		return
	}

	// Delete duplicates
	fmt.Println("\nDeleting duplicates...")
	deleted := 0
	errors := 0

	for _, record := range duplicatesToDelete {
		_, err := ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(record.ImageGUID)},
			},
		})
		if err != nil {
			fmt.Printf("  ERROR deleting %s: %v\n", record.ImageGUID, err)
			errors++
		} else {
			deleted++
		}
	}

	fmt.Printf("\nDeleted: %d\n", deleted)
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
