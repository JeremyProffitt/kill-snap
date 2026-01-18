// catchup.go - Script to find and reprocess unprocessed images in S3 incoming/
//
// Usage:
//   go run catchup.go                    # List unprocessed files and show stats
//   go run catchup.go -push              # Push unprocessed files to SQS (default limit: 100)
//   go run catchup.go -push -limit 500   # Push first 500 unprocessed files
//   go run catchup.go -push -nolimit     # Push ALL unprocessed files (no limit)
//   go run catchup.go -dry-run           # Show what would be pushed without actually pushing
//   go run catchup.go -watch             # Watch SQS queue and CloudWatch logs for errors
//   go run catchup.go -redrive           # Move DLQ messages back to main queue for retry
//   go run catchup.go -orphans           # Fix orphaned images (cross-reference S3 and DynamoDB)
//   go run catchup.go -migrate           # Migrate old file naming to new GUID format

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
)

// Default configuration for kill-snap project
const (
	defaultBucket       = "kill-snap"
	defaultTable        = "kill-snap-ImageMetadata"
	defaultQueueURL     = "https://sqs.us-east-2.amazonaws.com/759775734231/kill-snap-image-processing"
	defaultDLQURL       = "https://sqs.us-east-2.amazonaws.com/759775734231/kill-snap-image-processing-dlq"
	defaultRegion       = "us-east-2"
	defaultLimit        = 100
	defaultLogGroup     = "/aws/lambda/ImageThumbnailGenerator"
	watchPollInterval   = 30 * time.Second
	watchMaxDuration    = 2 * time.Hour
)

var (
	bucketName string
	tableName  string
	queueURL   string
	awsRegion  string
)

func init() {
	// Use environment variables if set, otherwise use defaults
	bucketName = getEnvOrDefault("BUCKET_NAME", defaultBucket)
	tableName = getEnvOrDefault("IMAGE_TABLE", defaultTable)
	queueURL = getEnvOrDefault("SQS_QUEUE_URL", defaultQueueURL)
	awsRegion = getEnvOrDefault("AWS_REGION", defaultRegion)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// S3Event represents the structure of an S3 event notification
type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

type S3EventRecord struct {
	EventVersion string   `json:"eventVersion"`
	EventSource  string   `json:"eventSource"`
	AWSRegion    string   `json:"awsRegion"`
	EventTime    string   `json:"eventTime"`
	EventName    string   `json:"eventName"`
	S3           S3Entity `json:"s3"`
}

type S3Entity struct {
	Bucket S3Bucket `json:"bucket"`
	Object S3Object `json:"object"`
}

type S3Bucket struct {
	Name string `json:"name"`
	ARN  string `json:"arn"`
}

type S3Object struct {
	Key  string `json:"key"`
	Size int64  `json:"size"`
}

// FileInfo stores info about a file in S3
type FileInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	BaseName     string
}

// Stats tracks processing statistics
type Stats struct {
	TotalIncoming    int
	JPGFiles         int
	RAWFiles         int
	OtherFiles       int
	AlreadyProcessed int
	Unprocessed      int
	Pushed           int
	Errors           int
}

// ImageRecord represents a DynamoDB image metadata record
type ImageRecord struct {
	ImageGUID        string `json:"ImageGUID" dynamodbav:"ImageGUID"`
	OriginalFile     string `json:"OriginalFile" dynamodbav:"OriginalFile"`
	OriginalFilename string `json:"OriginalFilename" dynamodbav:"OriginalFilename"`
	RawFile          string `json:"RawFile,omitempty" dynamodbav:"RawFile,omitempty"`
	Bucket           string `json:"Bucket" dynamodbav:"Bucket"`
	Thumbnail50      string `json:"Thumbnail50" dynamodbav:"Thumbnail50"`
	Thumbnail400     string `json:"Thumbnail400" dynamodbav:"Thumbnail400"`
	Reviewed         string `json:"Reviewed" dynamodbav:"Reviewed"`
}

// OrphanStats tracks orphan fixing statistics
type OrphanStats struct {
	TotalDynamoRecords    int
	TotalS3Images         int
	OrphanedDynamo        int // DynamoDB records without S3 files
	OrphanedS3            int // S3 files without DynamoDB records
	MislocatedS3          int // S3 files in wrong location
	Fixed                 int
	Errors                int
}

func main() {
	// Parse flags
	push := flag.Bool("push", false, "Push unprocessed files to SQS for reprocessing")
	dryRun := flag.Bool("dry-run", false, "Show what would be pushed without actually pushing")
	limit := flag.Int("limit", defaultLimit, "Limit the number of files to push (default: 100)")
	noLimit := flag.Bool("nolimit", false, "Process ALL unprocessed files (no limit)")
	verbose := flag.Bool("verbose", false, "Show detailed output for each file")
	watch := flag.Bool("watch", false, "Watch SQS queue and CloudWatch logs (press 'q' to quit)")
	redrive := flag.Bool("redrive", false, "Move DLQ messages back to main queue for retry")
	orphans := flag.Bool("orphans", false, "Fix orphaned images (cross-reference S3 and DynamoDB)")
	migrate := flag.Bool("migrate", false, "Migrate old file naming to new GUID format")
	flag.Parse()

	// If -nolimit is set, override the limit to 0 (unlimited)
	if *noLimit {
		*limit = 0
	}

	// Create AWS session with region
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))

	// Handle watch mode separately
	if *watch {
		runWatchMode(sess)
		return
	}

	// Handle redrive mode separately
	if *redrive {
		runRedriveMode(sess, *dryRun)
		return
	}

	// Handle orphans mode separately
	if *orphans {
		runOrphansMode(sess, *dryRun, *verbose)
		return
	}

	// Handle migrate mode separately
	if *migrate {
		runMigrateMode(sess, *dryRun, *verbose, *limit, *noLimit)
		return
	}

	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Table: %s\n", tableName)
	fmt.Printf("Queue: %s\n", queueURL)
	fmt.Printf("Region: %s\n", awsRegion)
	fmt.Println()
	s3Client := s3.New(sess)
	ddbClient := dynamodb.New(sess)
	sqsClient := sqs.New(sess)

	// Get all files from S3 incoming/
	fmt.Println("Scanning S3 incoming/ folder...")
	files, err := listIncomingFiles(s3Client)
	if err != nil {
		fmt.Printf("Error listing S3 files: %v\n", err)
		os.Exit(1)
	}

	stats := Stats{TotalIncoming: len(files)}
	fmt.Printf("Found %d files in incoming/\n\n", stats.TotalIncoming)

	// Categorize files
	var jpgFiles, rawFiles, otherFiles []FileInfo
	for _, f := range files {
		if isJPGFile(f.Key) {
			jpgFiles = append(jpgFiles, f)
			stats.JPGFiles++
		} else if isRAWFile(f.Key) {
			rawFiles = append(rawFiles, f)
			stats.RAWFiles++
		} else {
			otherFiles = append(otherFiles, f)
			stats.OtherFiles++
		}
	}

	fmt.Println("File breakdown:")
	fmt.Printf("  JPG files:   %d\n", stats.JPGFiles)
	fmt.Printf("  RAW files:   %d\n", stats.RAWFiles)
	fmt.Printf("  Other files: %d\n", stats.OtherFiles)
	fmt.Println()

	// Check which JPG files are already processed
	fmt.Println("Checking DynamoDB for already processed files...")
	var unprocessedFiles []FileInfo
	for i, f := range jpgFiles {
		if (i+1)%100 == 0 {
			fmt.Printf("  Checked %d/%d files...\n", i+1, len(jpgFiles))
		}

		processed, err := isAlreadyProcessed(ddbClient, f.BaseName)
		if err != nil {
			if *verbose {
				fmt.Printf("  Error checking %s: %v\n", f.Key, err)
			}
			stats.Errors++
			continue
		}

		if processed {
			stats.AlreadyProcessed++
			if *verbose {
				fmt.Printf("  [PROCESSED] %s\n", f.Key)
			}
		} else {
			stats.Unprocessed++
			unprocessedFiles = append(unprocessedFiles, f)
			if *verbose {
				fmt.Printf("  [UNPROCESSED] %s\n", f.Key)
			}
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total files in incoming/:     %d\n", stats.TotalIncoming)
	fmt.Printf("JPG files:                    %d\n", stats.JPGFiles)
	fmt.Printf("  - Already processed:        %d\n", stats.AlreadyProcessed)
	fmt.Printf("  - Unprocessed:              %d\n", stats.Unprocessed)
	fmt.Printf("RAW files:                    %d\n", stats.RAWFiles)
	fmt.Printf("Other files:                  %d\n", stats.OtherFiles)
	if stats.Errors > 0 {
		fmt.Printf("Errors during check:          %d\n", stats.Errors)
	}
	fmt.Println()

	// Show unprocessed files
	if len(unprocessedFiles) > 0 && !*push {
		fmt.Println("Unprocessed files (first 20):")
		showCount := 20
		if len(unprocessedFiles) < showCount {
			showCount = len(unprocessedFiles)
		}
		for i := 0; i < showCount; i++ {
			f := unprocessedFiles[i]
			fmt.Printf("  %s (%.2f MB, %s)\n", f.Key, float64(f.Size)/(1024*1024), f.LastModified.Format("2006-01-02 15:04"))
		}
		if len(unprocessedFiles) > showCount {
			fmt.Printf("  ... and %d more\n", len(unprocessedFiles)-showCount)
		}
		fmt.Println()
		fmt.Println("To reprocess these files, run:")
		fmt.Printf("  go run catchup.go -push              # Push first %d files (default limit)\n", defaultLimit)
		fmt.Println("  go run catchup.go -push -limit 500   # Push first 500 files")
		fmt.Println("  go run catchup.go -push -nolimit     # Push ALL files (no limit)")
		fmt.Println()
	}

	// Push to SQS if requested
	if *push && len(unprocessedFiles) > 0 {
		toPush := unprocessedFiles
		if *limit > 0 && len(toPush) > *limit {
			toPush = toPush[:*limit]
		}

		if *dryRun {
			fmt.Printf("DRY RUN: Would push %d files to SQS\n", len(toPush))
			for _, f := range toPush {
				fmt.Printf("  Would push: %s\n", f.Key)
			}
		} else {
			if *limit > 0 {
				fmt.Printf("Pushing %d files to SQS for reprocessing (limit: %d)...\n", len(toPush), *limit)
			} else {
				fmt.Printf("Pushing %d files to SQS for reprocessing (no limit)...\n", len(toPush))
			}

			for i, f := range toPush {
				if (i+1)%10 == 0 {
					fmt.Printf("  Pushed %d/%d files...\n", i+1, len(toPush))
				}

				err := pushToSQS(sqsClient, bucketName, f.Key, f.Size, awsRegion)
				if err != nil {
					fmt.Printf("  Error pushing %s: %v\n", f.Key, err)
					stats.Errors++
				} else {
					stats.Pushed++
					if *verbose {
						fmt.Printf("  Pushed: %s\n", f.Key)
					}
				}
			}

			fmt.Println()
			fmt.Printf("Successfully pushed %d files to SQS\n", stats.Pushed)
			if stats.Errors > 0 {
				fmt.Printf("Errors: %d\n", stats.Errors)
			}
		}
	}
}

// runWatchMode monitors SQS queue and CloudWatch logs
func runWatchMode(sess *session.Session) {
	sqsClient := sqs.New(sess)
	cwClient := cloudwatchlogs.New(sess)

	fmt.Println("=== Watch Mode ===")
	fmt.Printf("Monitoring SQS queue: %s\n", queueURL)
	fmt.Printf("Monitoring DLQ: %s\n", defaultDLQURL)
	fmt.Printf("Monitoring logs: %s\n", defaultLogGroup)
	fmt.Printf("Poll interval: %v\n", watchPollInterval)
	fmt.Printf("Max duration: %v\n", watchMaxDuration)
	fmt.Println()
	fmt.Println("Press 'q' + Enter to quit, or wait for queue to empty")
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()

	startTime := time.Now()
	lastLogTime := startTime.Add(-watchPollInterval) // Start by fetching recent logs
	consecutiveEmptyPolls := 0

	// Channel to listen for quit signal
	quitChan := make(chan bool)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))
			if input == "q" || input == "quit" {
				quitChan <- true
				return
			}
		}
	}()

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	// Do first poll immediately
	doWatchPoll(sqsClient, cwClient, &lastLogTime, &consecutiveEmptyPolls)

	for {
		select {
		case <-quitChan:
			fmt.Println("\nQuitting watch mode...")
			return

		case <-ticker.C:
			// Check max duration
			if time.Since(startTime) > watchMaxDuration {
				fmt.Printf("\nMax watch duration (%v) reached. Exiting.\n", watchMaxDuration)
				return
			}

			isEmpty := doWatchPoll(sqsClient, cwClient, &lastLogTime, &consecutiveEmptyPolls)

			// Exit if queue has been empty for 3 consecutive polls (1.5 minutes)
			if isEmpty {
				consecutiveEmptyPolls++
				if consecutiveEmptyPolls >= 3 {
					fmt.Println("\nQueue has been empty for 3 consecutive polls. Exiting.")
					return
				}
			} else {
				consecutiveEmptyPolls = 0
			}
		}
	}
}

// runRedriveMode moves messages from DLQ back to main queue
func runRedriveMode(sess *session.Session, dryRun bool) {
	sqsClient := sqs.New(sess)

	fmt.Println("=== DLQ Redrive Mode ===")
	fmt.Printf("Source (DLQ):  %s\n", defaultDLQURL)
	fmt.Printf("Target (Main): %s\n", queueURL)
	fmt.Println()

	// Get DLQ depth
	dlqDepth, _ := getQueueDepth(sqsClient, defaultDLQURL)
	if dlqDepth == 0 {
		fmt.Println("DLQ is empty. Nothing to redrive.")
		return
	}

	fmt.Printf("Found %d message(s) in DLQ\n", dlqDepth)
	fmt.Println()

	if dryRun {
		fmt.Println("DRY RUN: Would move messages from DLQ to main queue")
		fmt.Println("Run without -dry-run to actually move the messages")
		return
	}

	fmt.Println("Moving messages from DLQ to main queue...")
	fmt.Println("(Messages will be retried with full retry count)")
	fmt.Println()

	moved := 0
	errors := 0

	// Process messages in batches
	for {
		// Receive up to 10 messages from DLQ
		receiveResult, err := sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(defaultDLQURL),
			MaxNumberOfMessages: aws.Int64(10),
			WaitTimeSeconds:     aws.Int64(1), // Short wait since we know there are messages
			VisibilityTimeout:   aws.Int64(30),
		})
		if err != nil {
			fmt.Printf("Error receiving from DLQ: %v\n", err)
			break
		}

		if len(receiveResult.Messages) == 0 {
			break // No more messages
		}

		for _, msg := range receiveResult.Messages {
			// Send to main queue
			_, err := sqsClient.SendMessage(&sqs.SendMessageInput{
				QueueUrl:    aws.String(queueURL),
				MessageBody: msg.Body,
			})
			if err != nil {
				fmt.Printf("  Error sending to main queue: %v\n", err)
				errors++
				continue
			}

			// Delete from DLQ
			_, err = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(defaultDLQURL),
				ReceiptHandle: msg.ReceiptHandle,
			})
			if err != nil {
				fmt.Printf("  Error deleting from DLQ: %v\n", err)
				errors++
				continue
			}

			moved++
			if moved%10 == 0 {
				fmt.Printf("  Moved %d messages...\n", moved)
			}
		}
	}

	fmt.Println()
	fmt.Printf("=== Redrive Complete ===\n")
	fmt.Printf("Messages moved: %d\n", moved)
	if errors > 0 {
		fmt.Printf("Errors: %d\n", errors)
	}

	// Show new queue depths
	mainDepth, mainInFlight := getQueueDepth(sqsClient, queueURL)
	newDlqDepth, _ := getQueueDepth(sqsClient, defaultDLQURL)
	fmt.Println()
	fmt.Printf("Main queue: %d pending, %d in-flight\n", mainDepth, mainInFlight)
	fmt.Printf("DLQ: %d remaining\n", newDlqDepth)
}

// doWatchPoll performs one poll of SQS and CloudWatch
func doWatchPoll(sqsClient *sqs.SQS, cwClient *cloudwatchlogs.CloudWatchLogs, lastLogTime *time.Time, consecutiveEmpty *int) bool {
	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] ", timestamp)

	// Get main queue attributes
	queueDepth, inFlight := getQueueDepth(sqsClient, queueURL)
	dlqDepth, _ := getQueueDepth(sqsClient, defaultDLQURL)

	totalPending := queueDepth + inFlight

	// Display queue status
	statusIcon := "✓"
	if dlqDepth > 0 {
		statusIcon = "⚠"
	}
	if totalPending == 0 {
		statusIcon = "○"
	}

	fmt.Printf("%s Queue: %d pending, %d in-flight | DLQ: %d", statusIcon, queueDepth, inFlight, dlqDepth)

	if totalPending > 0 {
		// Estimate time remaining (rough: ~3 seconds per image)
		estimatedMins := (totalPending * 3) / 60
		if estimatedMins > 0 {
			fmt.Printf(" | ETA: ~%d min", estimatedMins)
		}
	}
	fmt.Println()

	// Fetch CloudWatch logs for errors since last poll
	errors := fetchErrorLogs(cwClient, *lastLogTime)
	*lastLogTime = time.Now()

	if len(errors) > 0 {
		fmt.Printf("    └─ Found %d error(s) in logs:\n", len(errors))
		for _, errMsg := range errors {
			// Truncate long messages
			if len(errMsg) > 120 {
				errMsg = errMsg[:117] + "..."
			}
			fmt.Printf("       • %s\n", errMsg)
		}
	}

	return queueDepth == 0 && inFlight == 0
}

// getQueueDepth returns the number of messages in queue and in-flight
func getQueueDepth(client *sqs.SQS, queueURL string) (int, int) {
	result, err := client.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(queueURL),
		AttributeNames: []*string{
			aws.String("ApproximateNumberOfMessages"),
			aws.String("ApproximateNumberOfMessagesNotVisible"),
		},
	})
	if err != nil {
		return 0, 0
	}

	pending := 0
	inFlight := 0

	if val, ok := result.Attributes["ApproximateNumberOfMessages"]; ok {
		pending, _ = strconv.Atoi(*val)
	}
	if val, ok := result.Attributes["ApproximateNumberOfMessagesNotVisible"]; ok {
		inFlight, _ = strconv.Atoi(*val)
	}

	return pending, inFlight
}

// fetchErrorLogs gets CloudWatch logs containing error or exception since given time
func fetchErrorLogs(client *cloudwatchlogs.CloudWatchLogs, since time.Time) []string {
	var errors []string

	// Search for error-related messages
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(defaultLogGroup),
		StartTime:     aws.Int64(since.UnixMilli()),
		FilterPattern: aws.String("?Error ?error ?Exception ?exception ?failed ?Failed"),
		Limit:         aws.Int64(20),
	}

	result, err := client.FilterLogEvents(input)
	if err != nil {
		// Silently ignore log fetch errors
		return errors
	}

	for _, event := range result.Events {
		msg := *event.Message
		// Skip normal log lines that just happen to contain these words in context
		if strings.Contains(msg, "REPORT RequestId") ||
			strings.Contains(msg, "INIT_START") ||
			strings.Contains(msg, "START RequestId") ||
			strings.Contains(msg, "END RequestId") {
			continue
		}
		// Skip the "Error processing RAW file" that's actually a normal retry
		if strings.Contains(msg, "will retry in 20 minutes") {
			continue
		}
		errors = append(errors, strings.TrimSpace(msg))
	}

	return errors
}

func listIncomingFiles(client *s3.S3) ([]FileInfo, error) {
	var files []FileInfo

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String("incoming/"),
	}

	err := client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			// Skip directories
			if strings.HasSuffix(key, "/") {
				continue
			}
			// Skip thumbnails
			if strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
				continue
			}

			files = append(files, FileInfo{
				Key:          key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				BaseName:     extractBaseName(key),
			})
		}
		return true
	})

	return files, err
}

func extractBaseName(key string) string {
	filename := filepath.Base(key)
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

func isJPGFile(key string) bool {
	lower := strings.ToLower(key)
	return strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")
}

func isRAWFile(key string) bool {
	lower := strings.ToLower(key)
	rawExtensions := []string{
		".cr2", ".cr3", ".nef", ".arw", ".raf", ".orf", ".dng",
		".rw2", ".pef", ".srw", ".3fr", ".raw", ".rwl", ".mrw",
		".nrw", ".kdc", ".dcr", ".sr2", ".erf", ".mef", ".mos",
	}
	for _, ext := range rawExtensions {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isAlreadyProcessed(client *dynamodb.DynamoDB, baseName string) (bool, error) {
	// Query by OriginalFilename using GSI
	result, err := client.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("OriginalFilenameIndex"),
		KeyConditionExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		// Fall back to scan if GSI doesn't exist
		return isAlreadyProcessedScan(client, baseName)
	}
	return len(result.Items) > 0, nil
}

func isAlreadyProcessedScan(client *dynamodb.DynamoDB, baseName string) (bool, error) {
	result, err := client.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		return false, err
	}
	return len(result.Items) > 0, nil
}

func pushToSQS(client *sqs.SQS, bucket, key string, size int64, region string) error {
	// Create an S3 event message
	s3Event := S3Event{
		Records: []S3EventRecord{
			{
				EventVersion: "2.1",
				EventSource:  "aws:s3",
				AWSRegion:    region,
				EventTime:    time.Now().UTC().Format(time.RFC3339),
				EventName:    "ObjectCreated:Put",
				S3: S3Entity{
					Bucket: S3Bucket{
						Name: bucket,
						ARN:  fmt.Sprintf("arn:aws:s3:::%s", bucket),
					},
					Object: S3Object{
						Key:  key,
						Size: size,
					},
				},
			},
		},
	}

	body, err := json.Marshal(s3Event)
	if err != nil {
		return fmt.Errorf("failed to marshal S3 event: %v", err)
	}

	_, err = client.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

// runOrphansMode cross-references S3 and DynamoDB to fix orphaned images
func runOrphansMode(sess *session.Session, dryRun, verbose bool) {
	s3Client := s3.New(sess)
	ddbClient := dynamodb.New(sess)

	fmt.Println("=== Orphan Detection Mode ===")
	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Table: %s\n", tableName)
	fmt.Println()

	stats := OrphanStats{}

	// Step 1: Scan all DynamoDB records
	fmt.Println("Step 1: Scanning DynamoDB records...")
	ddbRecords, err := scanAllDynamoRecords(ddbClient)
	if err != nil {
		fmt.Printf("Error scanning DynamoDB: %v\n", err)
		return
	}
	stats.TotalDynamoRecords = len(ddbRecords)
	fmt.Printf("  Found %d DynamoDB records\n", stats.TotalDynamoRecords)

	// Step 2: List all S3 files in images/ folder
	fmt.Println("Step 2: Scanning S3 images/ folder...")
	s3Files, err := listImagesFolder(s3Client)
	if err != nil {
		fmt.Printf("Error listing S3 files: %v\n", err)
		return
	}
	stats.TotalS3Images = len(s3Files)
	fmt.Printf("  Found %d S3 files in images/\n", stats.TotalS3Images)

	// Create lookup maps
	s3FileSet := make(map[string]bool)
	for _, f := range s3Files {
		s3FileSet[f.Key] = true
	}

	ddbByGUID := make(map[string]*ImageRecord)
	ddbByOriginalFile := make(map[string]*ImageRecord)
	for i := range ddbRecords {
		rec := &ddbRecords[i]
		ddbByGUID[rec.ImageGUID] = rec
		if rec.OriginalFile != "" {
			ddbByOriginalFile[rec.OriginalFile] = rec
		}
	}

	// Step 3: Find orphaned DynamoDB records (no corresponding S3 file)
	fmt.Println("\nStep 3: Finding orphaned DynamoDB records...")
	var orphanedDynamoRecords []ImageRecord
	for _, rec := range ddbRecords {
		// Check if the main image file exists
		if rec.OriginalFile != "" && !s3FileSet[rec.OriginalFile] {
			// Also check if thumbnails exist
			thumb50Exists := rec.Thumbnail50 != "" && s3FileSet[rec.Thumbnail50]
			thumb400Exists := rec.Thumbnail400 != "" && s3FileSet[rec.Thumbnail400]

			if !thumb50Exists && !thumb400Exists {
				orphanedDynamoRecords = append(orphanedDynamoRecords, rec)
				stats.OrphanedDynamo++
				if verbose {
					fmt.Printf("  [ORPHANED DDB] %s (GUID: %s)\n", rec.OriginalFile, rec.ImageGUID)
				}
			}
		}
	}
	fmt.Printf("  Found %d orphaned DynamoDB records\n", stats.OrphanedDynamo)

	// Step 4: Find orphaned S3 files (no corresponding DynamoDB record)
	fmt.Println("\nStep 4: Finding orphaned S3 files...")
	var orphanedS3Files []FileInfo
	for _, f := range s3Files {
		// Skip thumbnails - we only care about main images
		if strings.Contains(f.Key, ".50.") || strings.Contains(f.Key, ".400.") {
			continue
		}

		// Check if this file has a DynamoDB record
		if _, exists := ddbByOriginalFile[f.Key]; !exists {
			// Also try to match by GUID extracted from filename
			guid := extractGUIDFromKey(f.Key)
			if guid != "" {
				if _, exists := ddbByGUID[guid]; exists {
					continue // Found matching record by GUID
				}
			}
			orphanedS3Files = append(orphanedS3Files, f)
			stats.OrphanedS3++
			if verbose {
				fmt.Printf("  [ORPHANED S3] %s\n", f.Key)
			}
		}
	}
	fmt.Printf("  Found %d orphaned S3 files\n", stats.OrphanedS3)

	// Summary
	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Total DynamoDB records:     %d\n", stats.TotalDynamoRecords)
	fmt.Printf("Total S3 images:            %d\n", stats.TotalS3Images)
	fmt.Printf("Orphaned DynamoDB records:  %d\n", stats.OrphanedDynamo)
	fmt.Printf("Orphaned S3 files:          %d\n", stats.OrphanedS3)
	fmt.Println()

	if stats.OrphanedDynamo == 0 && stats.OrphanedS3 == 0 {
		fmt.Println("No orphans found. Everything is in sync!")
		return
	}

	// Fix orphans
	if dryRun {
		fmt.Println("DRY RUN: Would perform the following fixes:")
		if stats.OrphanedDynamo > 0 {
			fmt.Printf("  - Delete %d orphaned DynamoDB records\n", stats.OrphanedDynamo)
		}
		if stats.OrphanedS3 > 0 {
			fmt.Printf("  - Create DynamoDB records for %d orphaned S3 files (if thumbnails exist)\n", stats.OrphanedS3)
		}
		fmt.Println("\nRun without -dry-run to apply fixes")
		return
	}

	// Apply fixes
	fmt.Println("Applying fixes...")

	// Delete orphaned DynamoDB records
	if stats.OrphanedDynamo > 0 {
		fmt.Printf("\nDeleting %d orphaned DynamoDB records...\n", stats.OrphanedDynamo)
		for i, rec := range orphanedDynamoRecords {
			if (i+1)%50 == 0 {
				fmt.Printf("  Deleted %d/%d records...\n", i+1, len(orphanedDynamoRecords))
			}
			_, err := ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(tableName),
				Key: map[string]*dynamodb.AttributeValue{
					"ImageGUID": {S: aws.String(rec.ImageGUID)},
				},
			})
			if err != nil {
				fmt.Printf("  Error deleting %s: %v\n", rec.ImageGUID, err)
				stats.Errors++
			} else {
				stats.Fixed++
			}
		}
	}

	// Create DynamoDB records for orphaned S3 files that have thumbnails
	if stats.OrphanedS3 > 0 {
		fmt.Printf("\nProcessing %d orphaned S3 files...\n", stats.OrphanedS3)
		for i, f := range orphanedS3Files {
			if (i+1)%50 == 0 {
				fmt.Printf("  Processed %d/%d files...\n", i+1, len(orphanedS3Files))
			}

			// Check if thumbnails exist for this file
			basePath := strings.TrimSuffix(f.Key, filepath.Ext(f.Key))
			ext := filepath.Ext(f.Key)
			thumb50Key := basePath + ".50" + ext
			thumb400Key := basePath + ".400" + ext

			if s3FileSet[thumb50Key] && s3FileSet[thumb400Key] {
				// Thumbnails exist, create a DynamoDB record
				guid := extractGUIDFromKey(f.Key)
				if guid == "" {
					guid = uuid.New().String()
				}

				originalFilename := extractBaseName(f.Key)
				now := time.Now().UTC().Format(time.RFC3339)

				record := ImageRecord{
					ImageGUID:        guid,
					OriginalFile:     f.Key,
					OriginalFilename: originalFilename,
					Bucket:           bucketName,
					Thumbnail50:      thumb50Key,
					Thumbnail400:     thumb400Key,
					Reviewed:         "false",
				}

				item, err := dynamodbattribute.MarshalMap(record)
				if err != nil {
					fmt.Printf("  Error marshaling record for %s: %v\n", f.Key, err)
					stats.Errors++
					continue
				}

				// Add timestamps
				item["InsertedDateTime"] = &dynamodb.AttributeValue{S: aws.String(now)}
				item["UpdatedDateTime"] = &dynamodb.AttributeValue{S: aws.String(now)}

				_, err = ddbClient.PutItem(&dynamodb.PutItemInput{
					TableName: aws.String(tableName),
					Item:      item,
				})
				if err != nil {
					fmt.Printf("  Error creating record for %s: %v\n", f.Key, err)
					stats.Errors++
				} else {
					stats.Fixed++
					if verbose {
						fmt.Printf("  Created DynamoDB record for %s (GUID: %s)\n", f.Key, guid)
					}
				}
			} else {
				if verbose {
					fmt.Printf("  Skipping %s (no thumbnails found)\n", f.Key)
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("=== Fix Complete ===\n")
	fmt.Printf("Records fixed: %d\n", stats.Fixed)
	if stats.Errors > 0 {
		fmt.Printf("Errors: %d\n", stats.Errors)
	}
}

// runMigrateMode migrates old file naming to new GUID format
func runMigrateMode(sess *session.Session, dryRun, verbose bool, limit int, noLimit bool) {
	s3Client := s3.New(sess)
	_ = dynamodb.New(sess) // May be used for future validation
	sqsClient := sqs.New(sess)

	fmt.Println("=== File Migration Mode ===")
	fmt.Printf("Bucket: %s\n", bucketName)
	fmt.Printf("Table: %s\n", tableName)
	fmt.Println()
	fmt.Println("This will find files NOT in images/ folder and reprocess them with new GUID naming.")
	fmt.Println()

	// Find files that need migration (not in images/ folder, not in incoming/)
	fmt.Println("Scanning for files that need migration...")

	var toMigrate []FileInfo
	prefixes := []string{"approved/", "rejected/", "unreviewed/"}

	for _, prefix := range prefixes {
		files, err := listFilesWithPrefix(s3Client, prefix)
		if err != nil {
			fmt.Printf("Error listing %s: %v\n", prefix, err)
			continue
		}

		for _, f := range files {
			// Skip thumbnails
			if strings.Contains(f.Key, ".50.") || strings.Contains(f.Key, ".400.") {
				continue
			}
			// Only JPG files
			if isJPGFile(f.Key) {
				toMigrate = append(toMigrate, f)
			}
		}
	}

	fmt.Printf("Found %d files to migrate\n", len(toMigrate))

	if len(toMigrate) == 0 {
		fmt.Println("No files need migration. All files are already in the new format!")
		return
	}

	// Apply limit
	if !noLimit && limit > 0 && len(toMigrate) > limit {
		fmt.Printf("Limiting to first %d files (use -nolimit to process all)\n", limit)
		toMigrate = toMigrate[:limit]
	}

	fmt.Println()
	fmt.Println("Files to migrate (first 20):")
	showCount := 20
	if len(toMigrate) < showCount {
		showCount = len(toMigrate)
	}
	for i := 0; i < showCount; i++ {
		f := toMigrate[i]
		fmt.Printf("  %s (%.2f MB)\n", f.Key, float64(f.Size)/(1024*1024))
	}
	if len(toMigrate) > showCount {
		fmt.Printf("  ... and %d more\n", len(toMigrate)-showCount)
	}
	fmt.Println()

	if dryRun {
		fmt.Printf("DRY RUN: Would push %d files to SQS for reprocessing with new GUID naming\n", len(toMigrate))
		return
	}

	// Push files to SQS for reprocessing
	fmt.Printf("Pushing %d files to SQS for migration...\n", len(toMigrate))
	pushed := 0
	errors := 0

	for i, f := range toMigrate {
		if (i+1)%50 == 0 {
			fmt.Printf("  Pushed %d/%d files...\n", i+1, len(toMigrate))
		}

		err := pushToSQS(sqsClient, bucketName, f.Key, f.Size, awsRegion)
		if err != nil {
			if verbose {
				fmt.Printf("  Error pushing %s: %v\n", f.Key, err)
			}
			errors++
		} else {
			pushed++
		}
	}

	fmt.Println()
	fmt.Printf("=== Migration Started ===\n")
	fmt.Printf("Files pushed to SQS: %d\n", pushed)
	if errors > 0 {
		fmt.Printf("Errors: %d\n", errors)
	}
	fmt.Println("\nFiles will be reprocessed with new GUID naming format.")
	fmt.Println("Use -watch to monitor progress.")
}

// scanAllDynamoRecords scans all records from DynamoDB
func scanAllDynamoRecords(client *dynamodb.DynamoDB) ([]ImageRecord, error) {
	var records []ImageRecord
	var lastKey map[string]*dynamodb.AttributeValue

	for {
		input := &dynamodb.ScanInput{
			TableName:         aws.String(tableName),
			ExclusiveStartKey: lastKey,
		}

		result, err := client.Scan(input)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			var rec ImageRecord
			if err := dynamodbattribute.UnmarshalMap(item, &rec); err != nil {
				continue // Skip invalid records
			}
			records = append(records, rec)
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}

		fmt.Printf("  Scanned %d records so far...\n", len(records))
	}

	return records, nil
}

// listImagesFolder lists all files in the images/ folder
func listImagesFolder(client *s3.S3) ([]FileInfo, error) {
	return listFilesWithPrefix(client, "images/")
}

// listFilesWithPrefix lists all files with a given prefix
func listFilesWithPrefix(client *s3.S3, prefix string) ([]FileInfo, error) {
	var files []FileInfo

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}

	err := client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			key := *obj.Key
			// Skip directories
			if strings.HasSuffix(key, "/") {
				continue
			}

			files = append(files, FileInfo{
				Key:          key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				BaseName:     extractBaseName(key),
			})
		}
		return true
	})

	return files, err
}

// extractGUIDFromKey extracts a UUID from a file key like "images/abc-123-def.jpg"
func extractGUIDFromKey(key string) string {
	filename := filepath.Base(key)
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	// Remove thumbnail suffixes
	name = strings.TrimSuffix(name, ".50")
	name = strings.TrimSuffix(name, ".400")

	// Check if it looks like a UUID (36 chars with hyphens)
	if len(name) == 36 && strings.Count(name, "-") == 4 {
		return name
	}
	return ""
}
