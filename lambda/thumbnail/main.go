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
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	ImageGUID        string            `json:"ImageGUID"`
	OriginalFile     string            `json:"OriginalFile"`
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
	tableName      string
	openaiAPIKey   string
	circuitBreaker CircuitBreaker
)

const (
	circuitBreakerThreshold = 5                // Open circuit after 5 consecutive failures
	circuitBreakerReset     = 5 * time.Minute  // Reset circuit after 5 minutes
)

func init() {
	sess := session.Must(session.NewSession())
	s3Client = s3.New(sess)
	ddbClient = dynamodb.New(sess)
	tableName = os.Getenv("DYNAMODB_TABLE")
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

// checkIdempotency checks if an image with this S3 key has already been processed
func checkIdempotency(bucket, key string) (bool, string, error) {
	// Query by OriginalFile to see if we already processed this
	result, err := ddbClient.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("OriginalFile = :key"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":key": {S: aws.String(key)},
		},
		Limit: aws.Int64(1),
	})
	if err != nil {
		return false, "", err
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

func handler(ctx context.Context, sqsEvent events.SQSEvent) error {
	for _, sqsRecord := range sqsEvent.Records {
		// Parse S3 event from SQS message body
		var s3Event events.S3Event
		if err := json.Unmarshal([]byte(sqsRecord.Body), &s3Event); err != nil {
			fmt.Printf("Error parsing S3 event from SQS message: %v\n", err)
			// Return error to trigger retry via SQS visibility timeout
			return fmt.Errorf("failed to parse S3 event: %v", err)
		}

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

			// Skip if this is already a thumbnail
			if strings.Contains(key, ".50.") || strings.Contains(key, ".400.") {
				continue
			}

			// Idempotency check - skip if already processed
			alreadyProcessed, existingGUID, err := checkIdempotency(bucket, key)
			if err != nil {
				fmt.Printf("Warning: idempotency check failed: %v\n", err)
				// Continue processing on check failure
			} else if alreadyProcessed {
				fmt.Printf("Skipping already processed file: %s (GUID: %s)\n", key, existingGUID)
				continue
			}

			fmt.Printf("Processing file: %s from bucket: %s\n", key, bucket)

		// Download the original image
		result, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return fmt.Errorf("failed to get object: %v", err)
		}
		defer result.Body.Close()

		// Decode the image
		img, format, err := image.Decode(result.Body)
		if err != nil {
			if isCorruptedImageError(err) {
				fmt.Printf("Corrupted/unsupported image detected: %s - %v\n", key, err)
				result.Body.Close()
				if moveErr := moveToCorrupted(bucket, key); moveErr != nil {
					fmt.Printf("Warning: failed to move corrupted file: %v\n", moveErr)
				}
				continue // Skip to next file
			}
			return fmt.Errorf("failed to decode image: %v", err)
		}

		// Get original dimensions
		bounds := img.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()

		// Save file size before re-fetching for EXIF
		fileSize := int64(0)
		if result.ContentLength != nil {
			fileSize = *result.ContentLength
		}

		// Extract EXIF data
		result.Body.Close() // Close previous read
		result, err = s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		var exifData map[string]string
		if err != nil {
			fmt.Printf("Warning: failed to re-fetch image for EXIF extraction: %v\n", err)
			exifData = make(map[string]string)
		} else {
			// Read body into buffer for EXIF extraction
			exifBuf := new(bytes.Buffer)
			exifBuf.ReadFrom(result.Body)
			exifData = extractEXIF(bytes.NewReader(exifBuf.Bytes()))
			result.Body.Close()
		}

		// Generate thumbnails (150px for small icons, 800px for gallery/modal - higher quality)
		thumbnail50, err := generateThumbnail(img, 150)
		if err != nil {
			return fmt.Errorf("failed to generate 150px thumbnail: %v", err)
		}

		thumbnail400, err := generateThumbnail(img, 800)
		if err != nil {
			return fmt.Errorf("failed to generate 800px thumbnail: %v", err)
		}

		// Generate thumbnail file names
		ext := filepath.Ext(key)
		baseName := strings.TrimSuffix(key, ext)
		thumbnail50Key := baseName + ".50" + ext
		thumbnail400Key := baseName + ".400" + ext

		// Upload thumbnails to S3
		if err := uploadThumbnail(bucket, thumbnail50Key, thumbnail50); err != nil {
			return fmt.Errorf("failed to upload 50px thumbnail: %v", err)
		}

		if err := uploadThumbnail(bucket, thumbnail400Key, thumbnail400); err != nil {
			return fmt.Errorf("failed to upload 400px thumbnail: %v", err)
		}

		// Find related files with same base name
		relatedFiles, err := findRelatedFiles(bucket, baseName)
		if err != nil {
			fmt.Printf("Warning: failed to find related files: %v\n", err)
		}

		// Create metadata record
		now := time.Now().Format(time.RFC3339)
		metadata := ImageMetadata{
			ImageGUID:        uuid.New().String(),
			OriginalFile:     key,
			Bucket:           bucket,
			Thumbnail50:      thumbnail50Key,
			Thumbnail400:     thumbnail400Key,
			RelatedFiles:     relatedFiles,
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

		fmt.Printf("Successfully processed %s - GUID: %s\n", key, metadata.ImageGUID)
		fmt.Printf("Format: %s\n", format)

			// Generate AI keywords and description if OpenAI API key is configured
			if openaiAPIKey != "" {
				// Check circuit breaker before calling OpenAI
				if checkCircuitBreaker() {
					fmt.Printf("Circuit breaker OPEN - skipping GPT-4o analysis for %s\n", metadata.ImageGUID)
				} else {
					fmt.Printf("Starting GPT-4o analysis for new image %s\n", metadata.ImageGUID)

					// Use the thumbnail400 buffer for AI analysis
					aiResult, err := analyzeImageWithGPT4o(thumbnail400)
					if err != nil {
						fmt.Printf("GPT-4o analysis failed for image %s: %v\n", metadata.ImageGUID, err)
						recordOpenAIFailure()
						// Don't fail the whole operation, just log the error
					} else {
						recordOpenAISuccess()
						// Update the metadata with AI results
						if err := updateMetadataWithAI(metadata.ImageGUID, aiResult.Keywords, aiResult.Description); err != nil {
							fmt.Printf("Failed to update metadata with AI results for %s: %v\n", metadata.ImageGUID, err)
						} else {
							fmt.Printf("Successfully added AI keywords and description for %s\n", metadata.ImageGUID)
						}
					}
				}
			}
		} // end inner for loop (S3 records)
	} // end outer for loop (SQS records)

	return nil
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
