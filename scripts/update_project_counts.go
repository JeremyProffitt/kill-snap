// update_project_counts.go - Updates ImageCount for all projects based on actual DynamoDB records
//
// Usage:
//   go run update_project_counts.go                    # Dry run
//   go run update_project_counts.go -apply             # Apply updates

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const (
	defaultImageTable   = "kill-snap-ImageMetadata"
	defaultProjectTable = "kill-snap-Projects"
	awsRegion           = "us-east-2"
)

type Project struct {
	ProjectID  string `dynamodbav:"ProjectID"`
	S3Prefix   string `dynamodbav:"S3Prefix"`
	ImageCount int    `dynamodbav:"ImageCount"`
}

type ImageRecord struct {
	ImageGUID string `dynamodbav:"ImageGUID"`
	ProjectID string `dynamodbav:"ProjectID"`
	Status    string `dynamodbav:"Status"`
}

func main() {
	apply := flag.Bool("apply", false, "Apply the updates (default is dry-run)")
	flag.Parse()

	imageTable := getEnvOrDefault("IMAGE_TABLE", defaultImageTable)
	projectTable := getEnvOrDefault("PROJECT_TABLE", defaultProjectTable)

	fmt.Printf("Image Table: %s\n", imageTable)
	fmt.Printf("Project Table: %s\n", projectTable)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "APPLY", false: "DRY-RUN"}[*apply])
	fmt.Println()

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	ddbClient := dynamodb.New(sess)

	// Get all projects
	fmt.Println("Fetching projects...")
	var projects []Project
	var lastKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName: aws.String(projectTable),
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
			projects = append(projects, project)
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("Found %d projects\n\n", len(projects))

	// Count images per project
	fmt.Println("Counting images per project...")
	projectCounts := make(map[string]int)

	lastKey = nil
	for {
		input := &dynamodb.ScanInput{
			TableName:        aws.String(imageTable),
			FilterExpression: aws.String("#status = :project"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":project": {S: aws.String("project")},
			},
			ProjectionExpression: aws.String("ImageGUID, ProjectID"),
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := ddbClient.Scan(input)
		if err != nil {
			fmt.Printf("Error scanning images: %v\n", err)
			os.Exit(1)
		}

		for _, item := range result.Items {
			var record ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &record); err != nil {
				continue
			}
			if record.ProjectID != "" {
				projectCounts[record.ProjectID]++
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	// Update projects with incorrect counts
	updated := 0
	for _, project := range projects {
		actualCount := projectCounts[project.ProjectID]
		if project.ImageCount != actualCount {
			fmt.Printf("  %s (%s): %d -> %d\n", project.S3Prefix, project.ProjectID, project.ImageCount, actualCount)

			if *apply {
				_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
					TableName: aws.String(projectTable),
					Key: map[string]*dynamodb.AttributeValue{
						"ProjectID": {S: aws.String(project.ProjectID)},
					},
					UpdateExpression: aws.String("SET ImageCount = :count"),
					ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
						":count": {N: aws.String(fmt.Sprintf("%d", actualCount))},
					},
				})
				if err != nil {
					fmt.Printf("    ERROR: %v\n", err)
				}
			}
			updated++
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total projects: %d\n", len(projects))
	if *apply {
		fmt.Printf("Updated: %d\n", updated)
	} else {
		fmt.Printf("Would update: %d\n", updated)
		if updated > 0 {
			fmt.Println("\nThis was a DRY RUN. To apply updates, run:")
			fmt.Println("  go run update_project_counts.go -apply")
		}
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
