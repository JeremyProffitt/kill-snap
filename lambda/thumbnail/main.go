package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	ImageGUID        string            `json:"ImageGUID"`
	OriginalFile     string            `json:"OriginalFile"`     // S3 key (UUID-based: images/{uuid}.jpg)
	OriginalFilename string            `json:"OriginalFilename"` // Original base filename without extension (e.g., "IMG_0001")
	RawFile          string            `json:"RawFile,omitempty"` // S3 key of linked RAW file (e.g., images/{uuid}.CR2)
	Bucket           string            `json:"Bucket"`
	Thumbnail50      string            `json:"Thumbnail50"`
	Thumbnail400     string            `json:"Thumbnail400"`
	RelatedFiles     []string          `json:"RelatedFiles"`
	EXIFData         map[string]string `json:"EXIFData"`
	Width            int               `json:"Width"`
	Height           int               `json:"Height"`
	FileSize         int64             `json:"FileSize"`
	Reviewed         string            `json:"Reviewed"`
	Keywords         []string          `json:"Keywords,omitempty"`
	Description      string            `json:"Description,omitempty"`
	InsertedDateTime string            `json:"InsertedDateTime"`
	UpdatedDateTime  string            `json:"UpdatedDateTime"`
}

// OpenAI types for GPT-4o analysis
type OpenAIMessage struct {
	Role    string        `json:"role"`
	Content []interface{} `json:"content"`
}

type OpenAITextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type OpenAIImageContent struct {
	Type     string         `json:"type"`
	ImageURL OpenAIImageURL `json:"image_url"`
}

type OpenAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail"`
}

type OpenAIRequest struct {
	Model     string          `json:"model"`
	Messages  []OpenAIMessage `json:"messages"`
	MaxTokens int             `json:"max_tokens"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type AIAnalysisResult struct {
	Keywords    []string `json:"keywords"`
	Description string   `json:"description"`
}

// Circuit breaker state for OpenAI
type CircuitBreaker struct {
	consecutiveFailures int
	lastFailureTime     time.Time
	isOpen              bool
}

var (
	s3Client       *s3.S3
	ddbClient      *dynamodb.DynamoDB
	sqsClient      *sqs.SQS
	tableName      string
	sqsQueueURL    string
	openaiAPIKey   string
	circuitBreaker CircuitBreaker
)

const (
	circuitBreakerThreshold = 5               // Open circuit after 5 consecutive failures
	circuitBreakerReset     = 5 * time.Minute // Reset circuit after 5 minutes
	rawRetryDelaySeconds    = 1200            // 20 minutes delay for RAW file retries when JPG not ready
)

// Supported RAW file extensions (case-insensitive)
var rawExtensions = map[string]bool{
	".cr2": true, ".cr3": true, // Canon
	".nef": true, ".nrw": true, // Nikon
	".arw": true, ".srf": true, ".sr2": true, // Sony
	".orf": true,         // Olympus
	".rw2": true,         // Panasonic
	".raf": true,         // Fujifilm
	".dng": true,         // Adobe DNG / various
	".pef": true,         // Pentax
	".raw": true,         // Generic
	".rwl": true,         // Leica
	".3fr": true,         // Hasselblad
	".fff": true,         // Hasselblad
	".iiq": true,         // Phase One
	".erf": true,         // Epson
	".mrw": true,         // Minolta
	".x3f": true,         // Sigma
}

// isRawFile checks if the file extension indicates a RAW file
func isRawFile(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	return rawExtensions[ext]
}

// isJpgFile checks if the file extension indicates a JPEG file
func isJpgFile(key string) bool {
	ext := strings.ToLower(filepath.Ext(key))
	return ext == ".jpg" || ext == ".jpeg"
}

func init() {
	sess := session.Must(session.NewSession())
	s3Client = s3.New(sess)
	ddbClient = dynamodb.New(sess)
	sqsClient = sqs.New(sess)
	tableName = os.Getenv("DYNAMODB_TABLE")
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	circuitBreaker = CircuitBreaker{}
}

// checkCircuitBreaker returns true if OpenAI calls should be skipped
func checkCircuitBreaker() bool {
	if !circuitBreaker.isOpen {
		return false
	}
	// Check if enough time has passed to reset
	if time.Since(circuitBreaker.lastFailureTime) > circuitBreakerReset {
		fmt.Println("Circuit breaker reset - allowing OpenAI calls again")
		circuitBreaker.isOpen = false
		circuitBreaker.consecutiveFailures = 0
		return false
	}
	return true
}

// recordOpenAISuccess resets the circuit breaker on success
func recordOpenAISuccess() {
	circuitBreaker.consecutiveFailures = 0
	circuitBreaker.isOpen = false
}

// recordOpenAIFailure tracks failures and opens circuit if threshold reached
func recordOpenAIFailure() {
	circuitBreaker.consecutiveFailures++
	circuitBreaker.lastFailureTime = time.Now()
	if circuitBreaker.consecutiveFailures >= circuitBreakerThreshold {
		circuitBreaker.isOpen = true
		fmt.Printf("Circuit breaker OPEN - skipping OpenAI calls after %d consecutive failures\n", circuitBreaker.consecutiveFailures)
	}
}

// checkIdempotency checks if an image with this base filename has already been processed
// Uses the OriginalFilenameIndex GSI for efficient lookup
func checkIdempotency(bucket, key string) (bool, string, error) {
	// Extract base filename (without path and extension) for matching
	baseName := extractBaseName(key)

	// Query by OriginalFilename using GSI
	result, err := ddbClient.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("OriginalFilenameIndex"),
		KeyConditionExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		// Fall back to scan if GSI doesn't exist yet
		fmt.Printf("GSI query failed in idempotency check, falling back to scan: %v\n", err)
		return checkIdempotencyScan(baseName)
	}
	if len(result.Items) > 0 {
		// Already processed
		var meta ImageMetadata
		if err := dynamodbattribute.UnmarshalMap(result.Items[0], &meta); err == nil {
			return true, meta.ImageGUID, nil
		}
	}
	return false, "", nil
}

// checkIdempotencyScan is a fallback for when the GSI doesn't exist
func checkIdempotencyScan(baseName string) (bool, string, error) {
	result, err := ddbClient.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		return false, "", err
	}
	if len(result.Items) > 0 {
		var meta ImageMetadata
		if err := dynamodbattribute.UnmarshalMap(result.Items[0], &meta); err == nil {
			return true, meta.ImageGUID, nil
		}
	}
	return false, "", nil
}

// extractBaseName extracts the base filename without path and extension
// e.g., "incoming/photos/IMG_0001.jpg" -> "IMG_0001"
func extractBaseName(key string) string {
	// Get filename from path
	filename := filepath.Base(key)
	// Remove extension
	ext := filepath.Ext(filename)
	return strings.TrimSuffix(filename, ext)
}

// findRecordByOriginalFilename looks up an existing image record by its original base filename
// Used to link RAW files to their corresponding JPG records
// Uses the OriginalFilenameIndex GSI for efficient lookup
func findRecordByOriginalFilename(baseName string) (*ImageMetadata, error) {
	result, err := ddbClient.Query(&dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("OriginalFilenameIndex"),
		KeyConditionExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		// Fall back to scan if GSI doesn't exist yet (during transition)
		fmt.Printf("GSI query failed, falling back to scan: %v\n", err)
		return findRecordByOriginalFilenameScan(baseName)
	}
	if len(result.Items) == 0 {
		return nil, nil // Not found
	}
	var meta ImageMetadata
	if err := dynamodbattribute.UnmarshalMap(result.Items[0], &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// findRecordByOriginalFilenameScan is a fallback for when the GSI doesn't exist
func findRecordByOriginalFilenameScan(baseName string) (*ImageMetadata, error) {
	result, err := ddbClient.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("OriginalFilename = :baseName"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":baseName": {S: aws.String(baseName)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		return nil, err
	}
	if len(result.Items) == 0 {
		return nil, nil // Not found
	}
	var meta ImageMetadata
	if err := dynamodbattribute.UnmarshalMap(result.Items[0], &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// updateRecordWithRawFile updates an existing image record to link the RAW file
func updateRecordWithRawFile(imageGUID, rawFileKey string) error {
	now := time.Now().Format(time.RFC3339)
	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
		UpdateExpression: aws.String("SET RawFile = :rawFile, UpdatedDateTime = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":rawFile": {S: aws.String(rawFileKey)},
			":updated": {S: aws.String(now)},
		},
	})
	return err
}

// SoftRetryError indicates a message should be retried but isn't a real error
// (e.g., RAW file waiting for its JPG to be processed first)
type SoftRetryError struct {
	Message string
}

func (e *SoftRetryError) Error() string {
	return e.Message
}

func handler(ctx context.Context, sqsEvent events.SQSEvent) (events.SQSBatchResponse, error) {
	var batchItemFailures []events.SQSBatchItemFailure

	for _, sqsRecord := range sqsEvent.Records {
		// Parse S3 event from SQS message body
		var s3Event events.S3Event
		if err := json.Unmarshal([]byte(sqsRecord.Body), &s3Event); err != nil {
			fmt.Printf("Error parsing S3 event from SQS message: %v\n", err)
			// Add to batch failures for retry
			batchItemFailures = append(batchItemFailures, events.SQSBatchItemFailure{
				ItemIdentifier: sqsRecord.MessageId,
			})
			continue
		}

		var recordFailed bool
		for _, record := range s3Event.Records {
			bucket := record.S3.Bucket.Name
			key := record.S3.Object.Key

			// URL decode the key (S3 events have URL-encoded keys)
			decodedKey, err := urlDecode(key)
			if err != nil {
				fmt.Printf("Warning: could not decode key %s: %v\n", key, err)
				decodedKey = key
			}
			key = decodedKey

			// Skip if this is already in the images/ folder (already processed)
			if strings.HasPrefix(key, "images/") {
				continue
			}

			// Skip if this is already a thumbnail
			if strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
				continue
			}

			// Skip special folders
			if strings.HasPrefix(key, "deleted/") || strings.HasPrefix(key, "rejected/") ||
				strings.HasPrefix(key, "corrupted/") || strings.HasPrefix(key, "project-zips/") {
				continue
			}

			fmt.Printf("Processing file: %s from bucket: %s\n", key, bucket)

			// Route based on file type
			if isRawFile(key) {
				if err := processRawFile(bucket, key, sqsRecord.ReceiptHandle); err != nil {
					// Check if this is a soft retry (RAW waiting for JPG)
					if _, isSoftRetry := err.(*SoftRetryError); isSoftRetry {
						fmt.Printf("RAW file %s scheduled for retry (not an error)\n", key)
					} else {
						fmt.Printf("Error processing RAW file %s: %v\n", key, err)
					}
					recordFailed = true
				}
			} else if isJpgFile(key) {
				if err := processJpgFile(bucket, key); err != nil {
					fmt.Printf("Error processing JPG file %s: %v\n", key, err)
					recordFailed = true
				}
			} else {
				fmt.Printf("Skipping unsupported file type: %s\n", key)
			}
		} // end inner for loop (S3 records)

		// If any record in this SQS message failed, mark the whole message for retry
		if recordFailed {
			batchItemFailures = append(batchItemFailures, events.SQSBatchItemFailure{
				ItemIdentifier: sqsRecord.MessageId,
			})
		}
	} // end outer for loop (SQS records)

	return events.SQSBatchResponse{
		BatchItemFailures: batchItemFailures,
	}, nil
}

// processJpgFile handles JPG file processing with UUID-based naming
func processJpgFile(bucket, key string) error {
	// Extract original filename (without path and extension)
	originalFilename := extractBaseName(key)

	// Idempotency check - skip if already processed
	alreadyProcessed, existingGUID, err := checkIdempotency(bucket, key)
	if err != nil {
		fmt.Printf("Warning: idempotency check failed: %v\n", err)
		// Continue processing on check failure
	} else if alreadyProcessed {
		fmt.Printf("Skipping already processed file: %s (GUID: %s)\n", key, existingGUID)
		// Delete the original file since it's already processed
		deleteOriginalFile(bucket, key)
		return nil
	}

	// Generate UUID for this image
	imageGUID := uuid.New().String()

	// Download the original image
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get object: %v", err)
	}
	defer result.Body.Close()

	// Read the entire file into memory for multiple uses
	originalData, err := io.ReadAll(result.Body)
	if err != nil {
		return fmt.Errorf("failed to read file data: %v", err)
	}
	fileSize := int64(len(originalData))

	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(originalData))
	if err != nil {
		if isCorruptedImageError(err) {
			fmt.Printf("Corrupted/unsupported image detected: %s - %v\n", key, err)
			if moveErr := moveToCorrupted(bucket, key); moveErr != nil {
				fmt.Printf("Warning: failed to move corrupted file: %v\n", moveErr)
			}
			return nil // Don't fail, just skip
		}
		return fmt.Errorf("failed to decode image: %v", err)
	}

	// Get original dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Extract EXIF data
	exifData := extractEXIF(bytes.NewReader(originalData))

	// Generate thumbnails (150px for small icons, 800px for gallery/modal - higher quality)
	thumbnail50, err := generateThumbnail(img, 150)
	if err != nil {
		return fmt.Errorf("failed to generate 150px thumbnail: %v", err)
	}

	thumbnail400, err := generateThumbnail(img, 800)
	if err != nil {
		return fmt.Errorf("failed to generate 800px thumbnail: %v", err)
	}

	// Generate UUID-based file paths
	newJpgKey := fmt.Sprintf("images/%s.jpg", imageGUID)
	thumbnail50Key := fmt.Sprintf("images/%s.50.jpg", imageGUID)
	thumbnail400Key := fmt.Sprintf("images/%s.400.jpg", imageGUID)

	// Upload the original JPG to its new UUID-based location
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(newJpgKey),
		Body:        bytes.NewReader(originalData),
		ContentType: aws.String("image/jpeg"),
	})
	if err != nil {
		return fmt.Errorf("failed to copy JPG to new location: %v", err)
	}

	// Upload thumbnails to S3
	if err := uploadThumbnail(bucket, thumbnail50Key, thumbnail50); err != nil {
		return fmt.Errorf("failed to upload 50px thumbnail: %v", err)
	}

	if err := uploadThumbnail(bucket, thumbnail400Key, thumbnail400); err != nil {
		return fmt.Errorf("failed to upload 400px thumbnail: %v", err)
	}

	// Create metadata record
	now := time.Now().Format(time.RFC3339)
	metadata := ImageMetadata{
		ImageGUID:        imageGUID,
		OriginalFile:     newJpgKey,
		OriginalFilename: originalFilename,
		Bucket:           bucket,
		Thumbnail50:      thumbnail50Key,
		Thumbnail400:     thumbnail400Key,
		RelatedFiles:     []string{}, // We'll populate this differently now
		EXIFData:         exifData,
		Width:            width,
		Height:           height,
		FileSize:         fileSize,
		Reviewed:         "false",
		InsertedDateTime: now,
		UpdatedDateTime:  now,
	}

	// Store in DynamoDB
	if err := storeMetadata(metadata); err != nil {
		return fmt.Errorf("failed to store metadata: %v", err)
	}

	// Delete the original file from incoming location
	deleteOriginalFile(bucket, key)

	fmt.Printf("Successfully processed %s -> %s (GUID: %s, OriginalFilename: %s)\n",
		key, newJpgKey, imageGUID, originalFilename)
	fmt.Printf("Format: %s\n", format)

	// Generate AI keywords and description if OpenAI API key is configured
	if openaiAPIKey != "" {
		// Check circuit breaker before calling OpenAI
		if checkCircuitBreaker() {
			fmt.Printf("Circuit breaker OPEN - skipping GPT-4o analysis for %s\n", imageGUID)
		} else {
			fmt.Printf("Starting GPT-4o analysis for new image %s\n", imageGUID)

			// Use the thumbnail400 buffer for AI analysis
			aiResult, err := analyzeImageWithGPT4o(thumbnail400)
			if err != nil {
				fmt.Printf("GPT-4o analysis failed for image %s: %v\n", imageGUID, err)
				recordOpenAIFailure()
				// Don't fail the whole operation, just log the error
			} else {
				recordOpenAISuccess()
				// Update the metadata with AI results
				if err := updateMetadataWithAI(imageGUID, aiResult.Keywords, aiResult.Description); err != nil {
					fmt.Printf("Failed to update metadata with AI results for %s: %v\n", imageGUID, err)
				} else {
					fmt.Printf("Successfully added AI keywords and description for %s\n", imageGUID)
				}
			}
		}
	}

	return nil
}

// processRawFile handles RAW file processing - links to existing JPG record
// receiptHandle is used to set a longer visibility timeout if JPG isn't ready yet
func processRawFile(bucket, key, receiptHandle string) error {
	// Extract original filename (without path and extension)
	originalFilename := extractBaseName(key)
	rawExt := strings.ToLower(filepath.Ext(key))

	fmt.Printf("Processing RAW file: %s (base name: %s)\n", key, originalFilename)

	// Find the matching JPG record by original filename
	existingRecord, err := findRecordByOriginalFilename(originalFilename)
	if err != nil {
		return fmt.Errorf("failed to look up matching JPG record: %v", err)
	}

	if existingRecord == nil {
		// JPG hasn't been processed yet - set 20 minute visibility timeout and return error
		fmt.Printf("No matching JPG found for RAW file %s - setting 20 minute delay before retry\n", key)

		// Change message visibility to delay retry by 20 minutes
		if sqsQueueURL != "" && receiptHandle != "" {
			_, visErr := sqsClient.ChangeMessageVisibility(&sqs.ChangeMessageVisibilityInput{
				QueueUrl:          aws.String(sqsQueueURL),
				ReceiptHandle:     aws.String(receiptHandle),
				VisibilityTimeout: aws.Int64(rawRetryDelaySeconds),
			})
			if visErr != nil {
				fmt.Printf("Warning: failed to change message visibility: %v\n", visErr)
			} else {
				fmt.Printf("Set message visibility timeout to %d seconds for RAW retry\n", rawRetryDelaySeconds)
			}
		}

		return &SoftRetryError{Message: fmt.Sprintf("JPG not yet processed for RAW file %s, will retry in 20 minutes", originalFilename)}
	}

	// Check if RAW is already linked
	if existingRecord.RawFile != "" {
		fmt.Printf("RAW file already linked for %s: %s\n", originalFilename, existingRecord.RawFile)
		// Delete the original incoming RAW file
		deleteOriginalFile(bucket, key)
		return nil
	}

	// Download the RAW file
	result, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to get RAW file: %v", err)
	}
	defer result.Body.Close()

	// Read the RAW file data
	rawData, err := io.ReadAll(result.Body)
	if err != nil {
		return fmt.Errorf("failed to read RAW file data: %v", err)
	}

	// Generate the new RAW file key using the same UUID as the JPG
	newRawKey := fmt.Sprintf("images/%s%s", existingRecord.ImageGUID, rawExt)

	// Upload RAW to its new UUID-based location
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(newRawKey),
		Body:   bytes.NewReader(rawData),
	})
	if err != nil {
		return fmt.Errorf("failed to copy RAW to new location: %v", err)
	}

	// Update the DynamoDB record with the RAW file path
	if err := updateRecordWithRawFile(existingRecord.ImageGUID, newRawKey); err != nil {
		return fmt.Errorf("failed to update record with RAW file: %v", err)
	}

	// Delete the original RAW file from incoming location
	deleteOriginalFile(bucket, key)

	fmt.Printf("Successfully linked RAW file %s -> %s (GUID: %s)\n",
		key, newRawKey, existingRecord.ImageGUID)

	return nil
}

// deleteOriginalFile deletes a file from S3 (used after copying to new location)
func deleteOriginalFile(bucket, key string) {
	_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		fmt.Printf("Warning: failed to delete original file %s: %v\n", key, err)
	} else {
		fmt.Printf("Deleted original file: %s\n", key)
	}
}

// urlDecode decodes a URL-encoded string (S3 event keys are URL-encoded)
func urlDecode(s string) (string, error) {
	// Replace + with space, then decode percent-encoded characters
	s = strings.ReplaceAll(s, "+", " ")
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if hex, err := strconv.ParseInt(s[i+1:i+3], 16, 32); err == nil {
				result.WriteByte(byte(hex))
				i += 2
				continue
			}
		}
		result.WriteByte(s[i])
	}
	return result.String(), nil
}

func generateThumbnail(img image.Image, height int) (*bytes.Buffer, error) {
	// Resize maintaining aspect ratio using high-quality Lanczos resampling
	thumbnail := imaging.Resize(img, 0, height, imaging.Lanczos)

	// Apply mild sharpening to improve clarity after resize
	thumbnail = imaging.Sharpen(thumbnail, 0.5)

	// Encode to JPEG with good quality
	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, thumbnail, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}

	return buf, nil
}

func uploadThumbnail(bucket, key string, buf *bytes.Buffer) error {
	_, err := s3Client.PutObject(&s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("image/jpeg"),
	})
	return err
}

func extractEXIF(body *bytes.Reader) map[string]string {
	exifData := make(map[string]string)

	x, err := exif.Decode(body)
	if err != nil {
		fmt.Printf("Warning: failed to decode EXIF: %v\n", err)
		return exifData
	}

	// Extract common EXIF fields
	fields := []exif.FieldName{
		exif.Make,
		exif.Model,
		exif.DateTime,
		exif.DateTimeOriginal,
		exif.DateTimeDigitized,
		exif.Orientation,
		exif.XResolution,
		exif.YResolution,
		exif.Software,
		exif.Artist,
		exif.Copyright,
	}

	for _, field := range fields {
		tag, err := x.Get(field)
		if err == nil {
			exifData[string(field)] = tag.String()
		}
	}

	return exifData
}

func findRelatedFiles(bucket, baseName string) ([]string, error) {
	var relatedFiles []string

	// List objects with the same base name
	result, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(baseName),
	})
	if err != nil {
		return nil, err
	}

	for _, obj := range result.Contents {
		key := *obj.Key
		// Skip the original file and thumbnails
		if !strings.HasSuffix(key, ".jpg") || strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
			if key != baseName+".jpg" && key != baseName+".JPG" {
				relatedFiles = append(relatedFiles, key)
			}
		}
	}

	return relatedFiles, nil
}

func storeMetadata(metadata ImageMetadata) error {
	av, err := dynamodbattribute.MarshalMap(metadata)
	if err != nil {
		return err
	}

	_, err = ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	return err
}

// isCorruptedImageError checks if the error indicates a corrupted or unsupported image format
func isCorruptedImageError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	corruptedPatterns := []string{
		"unknown format",
		"short Huffman data",
		"invalid JPEG",
		"unexpected EOF",
		"bad RST marker",
		"missing SOI marker",
		"invalid header",
	}
	for _, pattern := range corruptedPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// moveToCorrupted moves a corrupted file to the corrupted/ prefix in S3
func moveToCorrupted(bucket, key string) error {
	// Create the destination key in corrupted/ folder
	corruptedKey := "corrupted/" + key

	// Copy the object to corrupted/ folder
	_, err := s3Client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		CopySource: aws.String(bucket + "/" + key),
		Key:        aws.String(corruptedKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy to corrupted/: %v", err)
	}

	// Delete the original object
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete original after moving to corrupted/: %v", err)
	}

	fmt.Printf("Moved corrupted file to: %s\n", corruptedKey)
	return nil
}

// OpenAI rate limit retry configuration
const (
	openaiMaxRetries     = 5
	openaiBaseRetryDelay = 2 * time.Second
	openaiMaxRetryDelay  = 60 * time.Second
)

// analyzeImageWithGPT4o sends the image to OpenAI GPT-4o for keyword and description generation
// Uses the thumbnail buffer directly instead of downloading from S3
func analyzeImageWithGPT4o(thumbnailBuf *bytes.Buffer) (*AIAnalysisResult, error) {
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Base64 encode the thumbnail image
	base64Image := base64.StdEncoding.EncodeToString(thumbnailBuf.Bytes())
	dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64Image)

	// Build OpenAI request
	prompt := `Analyze this photograph and provide:
1. A list of 10-15 relevant keywords for cataloging (single words or short phrases, lowercase)
2. A brief description (2-3 sentences) describing the image content, style, and mood

Respond in JSON format exactly like this:
{"keywords": ["keyword1", "keyword2", ...], "description": "Your description here."}`

	openaiReq := OpenAIRequest{
		Model: "gpt-4o",
		Messages: []OpenAIMessage{
			{
				Role: "user",
				Content: []interface{}{
					OpenAITextContent{
						Type: "text",
						Text: prompt,
					},
					OpenAIImageContent{
						Type: "image_url",
						ImageURL: OpenAIImageURL{
							URL:    dataURL,
							Detail: "high",
						},
					},
				},
			},
		},
		MaxTokens: 500,
	}

	reqBody, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request: %v", err)
	}

	// Make HTTP request to OpenAI with retry logic for rate limits
	var resp *http.Response
	var respBody []byte
	client := &http.Client{Timeout: 60 * time.Second}

	for attempt := 0; attempt <= openaiMaxRetries; attempt++ {
		httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %v", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+openaiAPIKey)

		resp, err = client.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("OpenAI API request failed: %v", err)
		}

		respBody, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read OpenAI response: %v", err)
		}

		// Check for rate limit (429) or server errors (5xx)
		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			if attempt < openaiMaxRetries {
				// Calculate delay with exponential backoff
				delay := openaiBaseRetryDelay * time.Duration(1<<uint(attempt))
				if delay > openaiMaxRetryDelay {
					delay = openaiMaxRetryDelay
				}

				// Check for Retry-After header
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
						delay = time.Duration(seconds) * time.Second
					}
				}

				fmt.Printf("OpenAI rate limited (status %d), retrying in %v (attempt %d/%d)\n",
					resp.StatusCode, delay, attempt+1, openaiMaxRetries)
				time.Sleep(delay)
				continue
			}
		}

		// Success or non-retryable error
		break
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %v", err)
	}

	if openaiResp.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openaiResp.Error.Message)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	// Parse the JSON response from GPT-4o
	content := openaiResp.Choices[0].Message.Content

	// Try to extract JSON from the response (it might be wrapped in markdown code blocks)
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var result AIAnalysisResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse GPT-4o response JSON: %v (content: %s)", err, content)
	}

	return &result, nil
}

// updateMetadataWithAI updates the image metadata in DynamoDB with AI-generated keywords and description
func updateMetadataWithAI(imageGUID string, keywords []string, description string) error {
	now := time.Now().Format(time.RFC3339)

	// Build update expression
	updateExpr := "SET UpdatedDateTime = :updated, Description = :desc"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":updated": {S: aws.String(now)},
		":desc":    {S: aws.String(description)},
	}

	// Add keywords if present
	if len(keywords) > 0 {
		updateExpr += ", Keywords = :keywords"
		keywordsList := make([]*dynamodb.AttributeValue, len(keywords))
		for i, kw := range keywords {
			keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
		}
		exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
	}

	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	})

	return err
}

func main() {
	lambda.Start(handler)
}
