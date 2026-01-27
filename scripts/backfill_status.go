// backfill_status.go - Script to backfill Status field for existing DynamoDB records
//
// This migration script sets the Status field on all existing ImageMetadata records
// that don't have it set, enabling efficient queries via the StatusIndex GSI.
//
// Usage:
//   go run backfill_status.go                    # Dry run - show what would be updated
//   go run backfill_status.go -apply             # Apply the updates
//   go run backfill_status.go -apply -verbose    # Apply with detailed output

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// Default configuration
const (
	defaultTable  = "kill-snap-ImageMetadata"
	defaultRegion = "us-east-2"
)

var (
	tableName string
	awsRegion string
)

func init() {
	tableName = getEnvOrDefault("IMAGE_TABLE", defaultTable)
	awsRegion = getEnvOrDefault("AWS_REGION", defaultRegion)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ImageRecord represents a DynamoDB image metadata record
type ImageRecord struct {
	ImageGUID   string `json:"ImageGUID" dynamodbav:"ImageGUID"`
	Reviewed    string `json:"Reviewed" dynamodbav:"Reviewed"`
	Status      string `json:"Status,omitempty" dynamodbav:"Status,omitempty"`
	GroupNumber int    `json:"GroupNumber,omitempty" dynamodbav:"GroupNumber,omitempty"`
	ProjectID   string `json:"ProjectID,omitempty" dynamodbav:"ProjectID,omitempty"`
}

// Stats tracks migration statistics
type Stats struct {
	TotalRecords    int
	AlreadyHasStatus int
	NeedsUpdate     int
	UpdatedToInbox  int
	UpdatedToApproved int
	UpdatedToRejected int
	UpdatedToProject int
	Errors          int
}

func main() {
	apply := flag.Bool("apply", false, "Apply the updates (default is dry-run)")
	verbose := flag.Bool("verbose", false, "Show detailed output for each record")
	flag.Parse()

	fmt.Printf("Table: %s\n", tableName)
	fmt.Printf("Region: %s\n", awsRegion)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	// Create AWS session
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)

	stats := Stats{}

	// Scan all records
	fmt.Println("Scanning all records...")
	var lastEvaluatedKey map[string]*dynamodb.AttributeValue
	pageNum := 0

	for {
		pageNum++
		input := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		}
		if lastEvaluatedKey != nil {
			input.ExclusiveStartKey = lastEvaluatedKey
		}

		result, err := ddbClient.Scan(input)
		if err != nil {
			fmt.Printf("Error scanning table: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("  Processing page %d (%d records)...\n", pageNum, len(result.Items))

		for _, item := range result.Items {
			var record ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &record); err != nil {
				fmt.Printf("    Error unmarshalling record: %v\n", err)
				stats.Errors++
				continue
			}

			stats.TotalRecords++

			// Skip records that already have a Status
			if record.Status != "" {
				stats.AlreadyHasStatus++
				if *verbose {
					fmt.Printf("    [SKIP] %s already has Status=%s\n", record.ImageGUID, record.Status)
				}
				continue
			}

			// Determine the correct Status
			newStatus := determineStatus(record)
			stats.NeedsUpdate++

			if *verbose {
				fmt.Printf("    [UPDATE] %s: Reviewed=%s, GroupNumber=%d, ProjectID=%s -> Status=%s\n",
					record.ImageGUID, record.Reviewed, record.GroupNumber, record.ProjectID, newStatus)
			}

			// Apply the update if not dry-run
			if *apply {
				err := updateStatus(ddbClient, record.ImageGUID, newStatus)
				if err != nil {
					fmt.Printf("    Error updating %s: %v\n", record.ImageGUID, err)
					stats.Errors++
					continue
				}
			}

			// Track by status type
			switch newStatus {
			case "inbox":
				stats.UpdatedToInbox++
			case "approved":
				stats.UpdatedToApproved++
			case "rejected":
				stats.UpdatedToRejected++
			case "project":
				stats.UpdatedToProject++
			}
		}

		// Check if there are more pages
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			break
		}
	}

	// Print summary
	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total records scanned:      %d\n", stats.TotalRecords)
	fmt.Printf("Already has Status:         %d\n", stats.AlreadyHasStatus)
	fmt.Printf("Needs Status update:        %d\n", stats.NeedsUpdate)
	fmt.Println()
	fmt.Println("Status breakdown:")
	fmt.Printf("  -> inbox:     %d\n", stats.UpdatedToInbox)
	fmt.Printf("  -> approved:  %d\n", stats.UpdatedToApproved)
	fmt.Printf("  -> rejected:  %d\n", stats.UpdatedToRejected)
	fmt.Printf("  -> project:   %d\n", stats.UpdatedToProject)
	if stats.Errors > 0 {
		fmt.Printf("Errors:                     %d\n", stats.Errors)
	}
	fmt.Println()

	if !*apply && stats.NeedsUpdate > 0 {
		fmt.Println("This was a DRY RUN. To apply these updates, run:")
		fmt.Println("  go run backfill_status.go -apply")
		fmt.Println()
	}

	if *apply && stats.NeedsUpdate > 0 {
		fmt.Printf("Successfully updated %d records.\n", stats.NeedsUpdate-stats.Errors)
	}
}

// determineStatus determines the correct Status based on record fields
func determineStatus(record ImageRecord) string {
	// If in a project, status is "project"
	if record.ProjectID != "" {
		return "project"
	}

	// If not reviewed, status is "inbox"
	if record.Reviewed == "false" || record.Reviewed == "" {
		return "inbox"
	}

	// If reviewed and has a group number > 0, status is "approved"
	if record.Reviewed == "true" && record.GroupNumber > 0 {
		return "approved"
	}

	// If reviewed but no group number, status is "rejected"
	if record.Reviewed == "true" && record.GroupNumber == 0 {
		return "rejected"
	}

	// Default to inbox for any edge cases
	return "inbox"
}

// updateStatus updates the Status field for a record
func updateStatus(ddbClient *dynamodb.DynamoDB, imageGUID, status string) error {
	now := time.Now().Format(time.RFC3339)

	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
		UpdateExpression: aws.String("SET #status = :status, UpdatedDateTime = :now"),
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":status": {S: aws.String(status)},
			":now":    {S: aws.String(now)},
		},
	})
	return err
}
