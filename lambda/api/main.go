package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	lambdasvc "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	sess              *session.Session
	ddbClient         *dynamodb.DynamoDB
	s3Client          *s3.S3
	lambdaClient      *lambdasvc.Lambda
	cwLogsClient      *cloudwatchlogs.CloudWatchLogs
	sqsClient         *sqs.SQS
	bucketName        string
	imageTable        string
	usersTable        string
	reviewGroupsTable string
	projectsTable     string
	adminUsername     string
	adminPassword     string
	functionName      string
	openaiAPIKey      string
	zipLambdaName     string
	sqsQueueURL       string
	sqsDLQURL         string
	jwtSecret         = []byte("kill-snap-secret-key-change-in-production")
)

func init() {
	sess = session.Must(session.NewSession())
	ddbClient = dynamodb.New(sess)
	s3Client = s3.New(sess)
	lambdaClient = lambdasvc.New(sess)
	cwLogsClient = cloudwatchlogs.New(sess)
	sqsClient = sqs.New(sess)
	bucketName = os.Getenv("BUCKET_NAME")
	imageTable = os.Getenv("IMAGE_TABLE")
	usersTable = os.Getenv("USERS_TABLE")
	reviewGroupsTable = os.Getenv("REVIEW_GROUPS_TABLE")
	projectsTable = os.Getenv("PROJECTS_TABLE")
	adminUsername = os.Getenv("ADMIN_USERNAME")
	adminPassword = os.Getenv("ADMIN_PASSWORD")
	functionName = os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
	zipLambdaName = os.Getenv("ZIP_LAMBDA_NAME")
	sqsQueueURL = os.Getenv("SQS_QUEUE_URL")
	sqsDLQURL = os.Getenv("SQS_DLQ_URL")

	// Initialize admin user if it doesn't exist
	if adminUsername != "" && adminPassword != "" {
		initializeAdminUser()
	}
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ImageResponse struct {
	ImageGUID        string            `json:"imageGUID"`
	OriginalFile     string            `json:"originalFile"`     // S3 key (UUID-based: images/{uuid}.jpg)
	OriginalFilename string            `json:"originalFilename,omitempty"` // Original base filename without extension
	RawFile          string            `json:"rawFile,omitempty"` // S3 key of linked RAW file
	Thumbnail50      string            `json:"thumbnail50"`
	Thumbnail400     string            `json:"thumbnail400"`
	Bucket           string            `json:"bucket"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	FileSize         int64             `json:"fileSize"`
	Reviewed         string            `json:"reviewed"`
	GroupNumber      int               `json:"groupNumber,omitempty"`
	ColorCode        string            `json:"colorCode,omitempty"`
	Rating           int               `json:"rating,omitempty"`
	Promoted         bool              `json:"promoted,omitempty"`
	Keywords         []string          `json:"keywords,omitempty"`
	Description      string            `json:"description,omitempty"` // AI-generated description
	EXIFData         map[string]string `json:"exifData,omitempty"`
	RelatedFiles     []string          `json:"relatedFiles,omitempty"`
	InsertedDateTime string            `json:"insertedDateTime,omitempty"`
	UpdatedDateTime  string            `json:"updatedDateTime,omitempty"`
	MoveStatus       string            `json:"moveStatus,omitempty"` // "pending", "moving", "complete", "failed"
	Status           string            `json:"status,omitempty"`     // "inbox", "approved", "rejected", "deleted", "project"
	ProjectID        string            `json:"projectId,omitempty"`
}

type UpdateImageRequest struct {
	GroupNumber int      `json:"groupNumber,omitempty"`
	ColorCode   string   `json:"colorCode,omitempty"`
	Rating      *int     `json:"rating,omitempty"`
	Promoted    bool     `json:"promoted,omitempty"`
	Reviewed    string   `json:"reviewed,omitempty"`
	Keywords    []string `json:"keywords,omitempty"`
}

type Project struct {
	ProjectID  string    `json:"projectId" dynamodbav:"ProjectID"`
	Name       string    `json:"name" dynamodbav:"Name"`
	S3Prefix   string    `json:"s3Prefix,omitempty" dynamodbav:"S3Prefix,omitempty"`
	CreatedAt  string    `json:"createdAt" dynamodbav:"CreatedAt"`
	ImageCount int       `json:"imageCount" dynamodbav:"ImageCount"`
	Keywords   []string  `json:"keywords,omitempty" dynamodbav:"Keywords,omitempty"`
	ZipFiles   []ZipFile `json:"zipFiles,omitempty" dynamodbav:"ZipFiles,omitempty"`
	Archived   bool      `json:"archived,omitempty" dynamodbav:"Archived,omitempty"`
}

// ZipFile represents a generated zip file for a project
type ZipFile struct {
	Key        string `json:"key" dynamodbav:"Key"`
	Size       int64  `json:"size" dynamodbav:"Size"`
	ImageCount int    `json:"imageCount" dynamodbav:"ImageCount"`
	CreatedAt  string `json:"createdAt" dynamodbav:"CreatedAt"`
	Status     string `json:"status" dynamodbav:"Status"` // "generating", "complete", "failed"
}

type CreateProjectRequest struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords,omitempty"`
}

type UpdateProjectRequest struct {
	Name     string   `json:"name,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Archived *bool    `json:"archived,omitempty"`
}

type AddToProjectRequest struct {
	All       bool   `json:"all,omitempty"`
	Group     int    `json:"group,omitempty"`
	ImageGUID string `json:"imageGUID,omitempty"`
}

// AsyncMoveRequest is used for async Lambda invocation to move files
type AsyncMoveRequest struct {
	Action      string `json:"action"` // "move_files"
	ImageGUID   string `json:"imageGUID"`
	DestPrefix  string `json:"destPrefix"`
	NewStatus   string `json:"newStatus"` // "approved", "rejected", "deleted"
	Bucket      string `json:"bucket"`
}

// OpenAI API types for GPT-4o vision analysis
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
	Detail string `json:"detail"` // "high" for high detail
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

// Retry configuration for DynamoDB operations
const (
	maxRetries     = 3
	baseRetryDelay = 250 * time.Millisecond
)

// retryableError checks if an error is retryable (throttling or transient server errors)
func retryableError(err error) bool {
	if err == nil {
		return false
	}
	if awsErr, ok := err.(awserr.Error); ok {
		switch awsErr.Code() {
		case dynamodb.ErrCodeProvisionedThroughputExceededException,
			dynamodb.ErrCodeRequestLimitExceeded,
			dynamodb.ErrCodeInternalServerError,
			"ThrottlingException",
			"ServiceUnavailable":
			return true
		}
	}
	return false
}

// withRetry executes a function with exponential backoff retry for transient errors
func withRetry[T any](operation func() (T, error)) (T, error) {
	var result T
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err = operation()
		if err == nil {
			return result, nil
		}

		if !retryableError(err) {
			return result, err
		}

		if attempt < maxRetries {
			delay := baseRetryDelay * time.Duration(1<<attempt) // 250ms, 500ms, 1000ms
			fmt.Printf("DynamoDB operation failed (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
		}
	}

	return result, fmt.Errorf("operation failed after %d retries: %w", maxRetries+1, err)
}

// withRetryNoResult is like withRetry but for operations that don't return a result
func withRetryNoResult(operation func() error) error {
	_, err := withRetry(func() (struct{}, error) {
		return struct{}{}, operation()
	})
	return err
}

func initializeAdminUser() {
	fmt.Printf("Initializing admin user: %s\n", adminUsername)

	// Check if admin user exists
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Username": {S: aws.String(adminUsername)},
		},
	})

	if err != nil {
		fmt.Printf("Error checking for admin user: %v\n", err)
		return
	}

	if result.Item == nil {
		fmt.Println("Admin user doesn't exist, creating...")
		// Create admin user
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Error hashing password: %v\n", err)
			return
		}

		item := map[string]*dynamodb.AttributeValue{
			"Username":     {S: aws.String(adminUsername)},
			"PasswordHash": {S: aws.String(string(hashedPassword))},
			"CreatedAt":    {S: aws.String(time.Now().Format(time.RFC3339))},
			"Role":         {S: aws.String("admin")},
		}

		_, err = ddbClient.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(usersTable),
			Item:      item,
		})

		if err != nil {
			fmt.Printf("Error creating admin user: %v\n", err)
		} else {
			fmt.Println("Admin user created successfully!")
		}
	} else {
		fmt.Println("Admin user already exists")
	}
}

// getImageDate extracts date from EXIF or falls back to InsertedDateTime
func getImageDate(img ImageResponse) time.Time {
	// Try EXIF DateTimeOriginal first
	if dateStr, ok := img.EXIFData["DateTimeOriginal"]; ok {
		cleaned := strings.Trim(dateStr, "\"")
		if t, err := time.Parse("2006:01:02 15:04:05", cleaned); err == nil {
			return t
		}
	}
	// Try DateTime
	if dateStr, ok := img.EXIFData["DateTime"]; ok {
		cleaned := strings.Trim(dateStr, "\"")
		if t, err := time.Parse("2006:01:02 15:04:05", cleaned); err == nil {
			return t
		}
	}
	// Fall back to InsertedDateTime
	if t, err := time.Parse(time.RFC3339, img.InsertedDateTime); err == nil {
		return t
	}
	return time.Now()
}

// buildDatePath returns YYYY/MM/DD format
func buildDatePath(t time.Time) string {
	return fmt.Sprintf("%d/%02d/%02d", t.Year(), int(t.Month()), t.Day())
}

// sanitizeS3Name converts a project name to an S3-safe prefix
// Rules: lowercase, alphanumeric + underscores, no consecutive underscores, max 63 chars
func sanitizeS3Name(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)

	// Replace spaces with underscores
	result = strings.ReplaceAll(result, " ", "_")

	// Replace any character that's not alphanumeric or underscore with underscore
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	result = reg.ReplaceAllString(result, "_")

	// Replace multiple consecutive underscores with single underscore
	reg = regexp.MustCompile(`_+`)
	result = reg.ReplaceAllString(result, "_")

	// Trim leading/trailing underscores
	result = strings.Trim(result, "_")

	// Limit to 63 characters (S3 prefix component limit)
	if len(result) > 63 {
		result = result[:63]
		result = strings.TrimRight(result, "_")
	}

	// Ensure we have something
	if result == "" {
		result = "project"
	}

	return result
}

// getProjectS3Prefix returns the S3 prefix for a project, preferring S3Prefix if set
func getProjectS3Prefix(project Project) string {
	if project.S3Prefix != "" {
		return project.S3Prefix
	}
	// Fallback to ProjectID for backward compatibility
	return project.ProjectID
}

// Common RAW file extensions
var rawExtensions = []string{
	".cr2", ".cr3", ".nef", ".arw", ".raf", ".orf", ".dng",
	".rw2", ".pef", ".srw", ".3fr", ".raw", ".rwl", ".mrw",
	".nrw", ".kdc", ".dcr", ".sr2", ".erf", ".mef", ".mos",
}

// findRawFiles looks for RAW files with the same base name as the original file
func findRawFiles(bucket string, originalFile string) []string {
	var rawFiles []string

	// Get the directory and base name without extension
	dir := filepath.Dir(originalFile)
	baseName := strings.TrimSuffix(filepath.Base(originalFile), filepath.Ext(originalFile))
	baseNameLower := strings.ToLower(baseName)

	// List objects in the same directory
	prefix := dir + "/"
	listResult, err := s3Client.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		fmt.Printf("Warning: failed to list S3 objects for raw file search: %v\n", err)
		return rawFiles
	}

	for _, obj := range listResult.Contents {
		key := *obj.Key
		fileName := filepath.Base(key)
		fileNameLower := strings.ToLower(fileName)
		fileExt := strings.ToLower(filepath.Ext(fileName))
		fileBaseName := strings.TrimSuffix(fileNameLower, fileExt)

		// Skip if it's the original file itself
		if key == originalFile {
			continue
		}

		// Check if base name matches and extension is a RAW format
		if fileBaseName == baseNameLower {
			for _, rawExt := range rawExtensions {
				if fileExt == rawExt {
					fmt.Printf("  Found RAW file: %s\n", key)
					rawFiles = append(rawFiles, key)
					break
				}
			}
		}
	}

	return rawFiles
}

// moveImageFiles moves original, thumbnails, and related files to new location
// Returns ErrSourceFileMissing if the original file doesn't exist in S3
func moveImageFiles(bucket string, img ImageResponse, destPrefix string) (map[string]string, error) {
	newPaths := make(map[string]string)

	// Move original file
	origFilename := filepath.Base(img.OriginalFile)
	newOriginal := destPrefix + "/" + origFilename
	// Skip if source and destination are the same (already in correct location)
	if img.OriginalFile != newOriginal {
		if err := copyS3Object(bucket, img.OriginalFile, newOriginal); err != nil {
			// Check if this is a NoSuchKey error (file doesn't exist)
			if isNoSuchKeyError(err) {
				return nil, ErrSourceFileMissing
			}
			return nil, fmt.Errorf("failed to copy original: %v", err)
		}
		deleteS3Object(bucket, img.OriginalFile)
	}
	newPaths["original"] = newOriginal

	// Move thumbnails
	thumb50Name := filepath.Base(img.Thumbnail50)
	newThumb50 := destPrefix + "/" + thumb50Name
	if img.Thumbnail50 != newThumb50 {
		copyS3Object(bucket, img.Thumbnail50, newThumb50)
		deleteS3Object(bucket, img.Thumbnail50)
	}
	newPaths["thumbnail50"] = newThumb50

	thumb400Name := filepath.Base(img.Thumbnail400)
	newThumb400 := destPrefix + "/" + thumb400Name
	if img.Thumbnail400 != newThumb400 {
		copyS3Object(bucket, img.Thumbnail400, newThumb400)
		deleteS3Object(bucket, img.Thumbnail400)
	}
	newPaths["thumbnail400"] = newThumb400

	// Find and move RAW files (same base name, different extension)
	rawFiles := findRawFiles(bucket, img.OriginalFile)
	var movedRawFiles []string
	for _, rawFile := range rawFiles {
		rawFilename := filepath.Base(rawFile)
		newRawPath := destPrefix + "/" + rawFilename
		// Skip if source and destination are the same
		if rawFile == newRawPath {
			movedRawFiles = append(movedRawFiles, newRawPath)
			continue
		}
		fmt.Printf("  Moving RAW file: %s -> %s\n", rawFile, newRawPath)
		if err := copyS3Object(bucket, rawFile, newRawPath); err != nil {
			fmt.Printf("  Warning: failed to copy RAW file %s: %v\n", rawFile, err)
			continue
		}
		deleteS3Object(bucket, rawFile)
		movedRawFiles = append(movedRawFiles, newRawPath)
	}
	newPaths["rawFiles"] = strings.Join(movedRawFiles, ",")

	// Move any existing related files
	for _, relFile := range img.RelatedFiles {
		relName := filepath.Base(relFile)
		newRelPath := destPrefix + "/" + relName
		// Skip if source and destination are the same
		if relFile != newRelPath {
			copyS3Object(bucket, relFile, newRelPath)
			deleteS3Object(bucket, relFile)
		}
	}

	return newPaths, nil
}

func copyS3Object(bucket, srcKey, dstKey string) error {
	return s3OperationWithRetry(func() error {
		_, err := s3Client.CopyObject(&s3.CopyObjectInput{
			Bucket:     aws.String(bucket),
			CopySource: aws.String(url.PathEscape(bucket + "/" + srcKey)),
			Key:        aws.String(dstKey),
		})
		return err
	})
}

func deleteS3Object(bucket, key string) {
	s3OperationWithRetry(func() error {
		_, err := s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		return err
	})
}

// s3OperationWithRetry executes an S3 operation with exponential backoff retry
func s3OperationWithRetry(operation func() error) error {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		lastErr = operation()
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable (throttling, service unavailable)
		if !isRetryableS3Error(lastErr) {
			return lastErr
		}

		// Exponential backoff: 100ms, 200ms, 400ms
		delay := baseDelay * time.Duration(1<<attempt)
		fmt.Printf("S3 operation failed (attempt %d/%d): %v, retrying in %v\n", attempt+1, maxRetries, lastErr, delay)
		time.Sleep(delay)
	}
	return lastErr
}

// isRetryableS3Error checks if an S3 error is retryable
func isRetryableS3Error(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"throttl",
		"slow down",
		"service unavailable",
		"internal error",
		"connection reset",
		"timeout",
	}
	for _, pattern := range retryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// isNoSuchKeyError checks if an error is an S3 NoSuchKey error
func isNoSuchKeyError(err error) bool {
	if err == nil {
		return false
	}
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		return awsErr.Code() == s3.ErrCodeNoSuchKey || awsErr.Code() == "NotFound"
	}
	return strings.Contains(err.Error(), "NoSuchKey")
}

// s3ObjectExists checks if an object exists in S3
func s3ObjectExists(bucket, key string) bool {
	_, err := s3Client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err == nil
}

// ErrSourceFileMissing indicates the source file doesn't exist in S3
var ErrSourceFileMissing = errors.New("source file missing from S3")

// deleteImageFromDB removes an image record from DynamoDB and decrements project ImageCount if applicable
func deleteImageFromDB(imageGUID string) error {
	// First get the image to check if it belongs to a project
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
		ProjectionExpression: aws.String("ProjectID"),
	})
	if err != nil {
		return err
	}

	// Extract ProjectID if present
	var projectID string
	if result.Item != nil {
		if pidAttr, ok := result.Item["ProjectID"]; ok && pidAttr.S != nil {
			projectID = *pidAttr.S
		}
	}

	// Delete the image record
	_, err = ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageGUID)},
		},
	})
	if err != nil {
		return err
	}

	// Decrement project ImageCount if image belonged to a project
	if projectID != "" {
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(projectsTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ProjectID": {S: aws.String(projectID)},
			},
			UpdateExpression: aws.String("ADD ImageCount :dec"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":dec": {N: aws.String("-1")},
			},
		})
		if err != nil {
			fmt.Printf("Warning: Failed to decrement project ImageCount for %s: %v\n", projectID, err)
		}
	}

	return nil
}

// OpenAI rate limit retry configuration
const (
	openaiMaxRetries     = 5
	openaiBaseRetryDelay = 2 * time.Second
	openaiMaxRetryDelay  = 60 * time.Second
)

// analyzeImageWithGPT4o sends the image to OpenAI GPT-4o for keyword and description generation
func analyzeImageWithGPT4o(bucket, thumbnailKey string) (*AIAnalysisResult, error) {
	if openaiAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Download thumbnail from S3
	getResult, err := s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(thumbnailKey),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download thumbnail: %v", err)
	}
	defer getResult.Body.Close()

	imageData, err := io.ReadAll(getResult.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read thumbnail data: %v", err)
	}

	// Determine MIME type from extension
	mimeType := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(thumbnailKey), ".png") {
		mimeType = "image/png"
	}

	// Base64 encode the image
	base64Image := base64.StdEncoding.EncodeToString(imageData)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Image)

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
					if seconds, err := strconv.Atoi(retryAfter); err == nil {
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
		// If JSON parsing fails, try to extract what we can
		fmt.Printf("Failed to parse GPT-4o JSON response: %v, content: %s\n", err, content)
		return nil, fmt.Errorf("failed to parse GPT-4o response as JSON: %v", err)
	}

	fmt.Printf("GPT-4o analysis complete: %d keywords, description length: %d\n", len(result.Keywords), len(result.Description))
	return &result, nil
}

// handleAsyncMoveFiles processes async file move requests
func handleAsyncMoveFiles(req AsyncMoveRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Async move started for image %s to %s\n", req.ImageGUID, req.DestPrefix)

	// Update status to "moving"
	err := withRetryNoResult(func() error {
		_, updateErr := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
			UpdateExpression: aws.String("SET MoveStatus = :status"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("moving")},
			},
		})
		return updateErr
	})
	if err != nil {
		fmt.Printf("Error updating move status to moving: %v\n", err)
	}

	// Get current image metadata
	getResult, err := withRetry(func() (*dynamodb.GetItemOutput, error) {
		return ddbClient.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
		})
	})
	if err != nil || getResult.Item == nil {
		// Update status to failed
		withRetryNoResult(func() error {
			_, updateErr := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
				TableName: aws.String(imageTable),
				Key: map[string]*dynamodb.AttributeValue{
					"ImageGUID": {S: aws.String(req.ImageGUID)},
				},
				UpdateExpression: aws.String("SET MoveStatus = :status"),
				ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
					":status": {S: aws.String("failed")},
				},
			})
			return updateErr
		})
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: `{"success": false, "error": "Image not found"}`}, nil
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(getResult.Item, &img)

	// Move the files
	newPaths, err := moveImageFiles(req.Bucket, img, req.DestPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		// If the source file is missing from S3, delete the image record from DynamoDB
		if errors.Is(err, ErrSourceFileMissing) {
			fmt.Printf("Source file missing for image %s, deleting from database\n", req.ImageGUID)
			if delErr := deleteImageFromDB(req.ImageGUID); delErr != nil {
				fmt.Printf("Error deleting image from DB: %v\n", delErr)
			}
			return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: `{"success": true, "deleted": true, "reason": "source file missing"}`}, nil
		}
		// Update status to failed
		withRetryNoResult(func() error {
			_, updateErr := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
				TableName: aws.String(imageTable),
				Key: map[string]*dynamodb.AttributeValue{
					"ImageGUID": {S: aws.String(req.ImageGUID)},
				},
				UpdateExpression: aws.String("SET MoveStatus = :status"),
				ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
					":status": {S: aws.String("failed")},
				},
			})
			return updateErr
		})
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: fmt.Sprintf(`{"success": false, "error": "%v"}`, err)}, nil
	}

	// Update DynamoDB with new paths and complete status
	err = withRetryNoResult(func() error {
		_, updateErr := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
			UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :newStatus, MoveStatus = :moveStatus, UpdatedDateTime = :updated"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":orig":       {S: aws.String(newPaths["original"])},
				":t50":        {S: aws.String(newPaths["thumbnail50"])},
				":t400":       {S: aws.String(newPaths["thumbnail400"])},
				":newStatus":  {S: aws.String(req.NewStatus)},
				":moveStatus": {S: aws.String("complete")},
				":updated":    {S: aws.String(time.Now().Format(time.RFC3339))},
			},
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
		})
		return updateErr
	})
	if err != nil {
		fmt.Printf("Error updating image after move: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: fmt.Sprintf(`{"success": false, "error": "%v"}`, err)}, nil
	}

	// For approved images, analyze with GPT-4o to generate keywords and description
	if req.NewStatus == "approved" && openaiAPIKey != "" {
		fmt.Printf("Starting GPT-4o analysis for approved image %s\n", req.ImageGUID)

		// Use the new thumbnail path after move
		aiResult, err := analyzeImageWithGPT4o(req.Bucket, newPaths["thumbnail400"])
		if err != nil {
			fmt.Printf("GPT-4o analysis failed for image %s: %v\n", req.ImageGUID, err)
			// Don't fail the whole operation, just log the error
		} else {
			// Merge AI keywords with existing user keywords (case-insensitive deduplication)
			existingKeywordsLower := make(map[string]bool)
			for _, kw := range img.Keywords {
				existingKeywordsLower[strings.ToLower(kw)] = true
			}

			mergedKeywords := append([]string{}, img.Keywords...) // Start with existing
			for _, kw := range aiResult.Keywords {
				if !existingKeywordsLower[strings.ToLower(kw)] {
					mergedKeywords = append(mergedKeywords, kw)
					existingKeywordsLower[strings.ToLower(kw)] = true
				}
			}

			// Build update expression for keywords and description
			updateExpr := "SET #desc = :desc, UpdatedDateTime = :updated"
			exprAttrValues := map[string]*dynamodb.AttributeValue{
				":desc":    {S: aws.String(aiResult.Description)},
				":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
			}
			exprAttrNames := map[string]*string{
				"#desc": aws.String("Description"),
			}

			if len(mergedKeywords) > 0 {
				keywordsList := make([]*dynamodb.AttributeValue, len(mergedKeywords))
				for i, kw := range mergedKeywords {
					keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
				}
				updateExpr += ", Keywords = :keywords"
				exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
			}

			updateErr := withRetryNoResult(func() error {
				_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
					TableName: aws.String(imageTable),
					Key: map[string]*dynamodb.AttributeValue{
						"ImageGUID": {S: aws.String(req.ImageGUID)},
					},
					UpdateExpression:          aws.String(updateExpr),
					ExpressionAttributeValues: exprAttrValues,
					ExpressionAttributeNames:  exprAttrNames,
				})
				return err
			})
			if updateErr != nil {
				fmt.Printf("Error updating image with AI analysis: %v\n", updateErr)
			} else {
				fmt.Printf("GPT-4o analysis saved for image %s: %d keywords\n", req.ImageGUID, len(mergedKeywords))
			}
		}
	}

	fmt.Printf("Async move completed for image %s\n", req.ImageGUID)
	return events.APIGatewayProxyResponse{StatusCode: 200, Headers: headers, Body: `{"success": true}`}, nil
}

// triggerAsyncMove invokes Lambda asynchronously to move files
func triggerAsyncMove(imageGUID, destPrefix, newStatus, bucket string) error {
	payload, err := json.Marshal(AsyncMoveRequest{
		Action:     "move_files",
		ImageGUID:  imageGUID,
		DestPrefix: destPrefix,
		NewStatus:  newStatus,
		Bucket:     bucket,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal async request: %v", err)
	}

	// Wrap payload in API Gateway format for the handler
	wrappedPayload, _ := json.Marshal(map[string]string{
		"body": string(payload),
	})

	_, err = lambdaClient.Invoke(&lambdasvc.InvokeInput{
		FunctionName:   aws.String(functionName),
		InvocationType: aws.String("Event"), // Async invocation
		Payload:        wrappedPayload,
	})
	if err != nil {
		return fmt.Errorf("failed to invoke lambda: %v", err)
	}

	return nil
}

func getColorName(groupNumber int) string {
	// Matches Lightroom color labels: Red, Yellow, Green, Blue, Purple
	colors := map[int]string{
		0: "none", 1: "red", 2: "yellow", 3: "green", 4: "blue", 5: "purple",
	}
	if name, ok := colors[groupNumber]; ok {
		return name
	}
	return "none"
}

// ScheduledEvent represents an EventBridge scheduled event
type ScheduledEvent struct {
	Source     string `json:"source"`
	DetailType string `json:"detail-type"`
	Detail     struct {
		Action string `json:"action"`
	} `json:"detail"`
}

// BackfillConfig controls the nightly keyword backfill
const (
	backfillBatchSize    = 50 // Max images to process per run
	backfillDelayBetween = 2 * time.Second // Delay between API calls
)

// handleBackfillKeywords processes images without keywords
func handleBackfillKeywords() error {
	if openaiAPIKey == "" {
		fmt.Println("Backfill: OpenAI API key not configured, skipping")
		return nil
	}

	fmt.Println("Starting keyword backfill for images without keywords...")

	// Scan for images without description (which means no AI analysis was done)
	// We scan for Description being empty or not existing
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(imageTable),
		FilterExpression: aws.String("attribute_not_exists(Description) OR Description = :empty"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":empty": {S: aws.String("")},
		},
		Limit: aws.Int64(backfillBatchSize),
	}

	result, err := ddbClient.Scan(scanInput)
	if err != nil {
		return fmt.Errorf("failed to scan for images without keywords: %v", err)
	}

	fmt.Printf("Backfill: Found %d images without keywords to process\n", len(result.Items))

	processedCount := 0
	errorCount := 0

	for _, item := range result.Items {
		var img ImageResponse
		if err := dynamodbattribute.UnmarshalMap(item, &img); err != nil {
			fmt.Printf("Backfill: Failed to unmarshal image: %v\n", err)
			errorCount++
			continue
		}

		fmt.Printf("Backfill: Processing image %s (%s)\n", img.ImageGUID, img.OriginalFile)

		// Call GPT-4o to analyze the image
		aiResult, err := analyzeImageWithGPT4o(img.Bucket, img.Thumbnail400)
		if err != nil {
			fmt.Printf("Backfill: AI analysis failed for %s: %v\n", img.ImageGUID, err)
			errorCount++
			// Continue to next image instead of stopping
			time.Sleep(backfillDelayBetween)
			continue
		}

		// Update the image with AI results
		now := time.Now().Format(time.RFC3339)
		updateExpr := "SET UpdatedDateTime = :updated, Description = :desc"
		exprAttrValues := map[string]*dynamodb.AttributeValue{
			":updated": {S: aws.String(now)},
			":desc":    {S: aws.String(aiResult.Description)},
		}

		if len(aiResult.Keywords) > 0 {
			updateExpr += ", Keywords = :keywords"
			keywordsList := make([]*dynamodb.AttributeValue, len(aiResult.Keywords))
			for i, kw := range aiResult.Keywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		}

		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(img.ImageGUID)},
			},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeValues: exprAttrValues,
		})

		if err != nil {
			fmt.Printf("Backfill: Failed to update image %s: %v\n", img.ImageGUID, err)
			errorCount++
		} else {
			fmt.Printf("Backfill: Successfully added keywords to %s\n", img.ImageGUID)
			processedCount++
		}

		// Add delay between API calls to avoid rate limits
		time.Sleep(backfillDelayBetween)
	}

	fmt.Printf("Backfill complete: %d processed, %d errors\n", processedCount, errorCount)
	return nil
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
	}

	// Check if this is a scheduled event (EventBridge)
	// When invoked by EventBridge, the request will have empty HTTP method and path
	// but the body will contain the scheduled event details
	if request.HTTPMethod == "" && request.Path == "" && request.Body != "" {
		var scheduledEvent ScheduledEvent
		if err := json.Unmarshal([]byte(request.Body), &scheduledEvent); err == nil {
			if scheduledEvent.Source == "aws.events" || scheduledEvent.DetailType == "Scheduled Event" {
				fmt.Println("Received scheduled event, running keyword backfill...")
				if err := handleBackfillKeywords(); err != nil {
					fmt.Printf("Backfill error: %v\n", err)
					return events.APIGatewayProxyResponse{
						StatusCode: 500,
						Body:       fmt.Sprintf(`{"error": "%s"}`, err.Error()),
					}, nil
				}
				return events.APIGatewayProxyResponse{
					StatusCode: 200,
					Body:       `{"message": "Backfill completed"}`,
				}, nil
			}
		}
	}

	// Check if this is an async move request (direct Lambda invocation)
	if request.Body != "" && request.HTTPMethod == "" && request.Path == "" {
		var asyncReq AsyncMoveRequest
		if err := json.Unmarshal([]byte(request.Body), &asyncReq); err == nil && asyncReq.Action == "move_files" {
			return handleAsyncMoveFiles(asyncReq, headers)
		}
	}

	// Handle OPTIONS requests for CORS
	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
		}, nil
	}

	path := request.Path
	method := request.HTTPMethod

	// Login endpoint doesn't require authentication
	if path == "/api/login" && method == "POST" {
		return handleLogin(request, headers)
	}

	// All other endpoints require authentication
	token := extractToken(request.Headers)
	if token == "" {
		return errorResponse(401, "Unauthorized", headers)
	}

	if !validateToken(token) {
		return errorResponse(401, "Invalid token", headers)
	}

	// Route requests
	switch {
	case path == "/api/stats" && method == "GET":
		return handleGetStats(headers)
	case path == "/api/images" && method == "GET":
		return handleListImages(request, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "PUT":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleUpdateImage(imageID, request, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/download") && method == "GET":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/download")
		return handleDownload(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/regenerate-ai") && method == "POST":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/regenerate-ai")
		return handleRegenerateAI(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/undelete") && method == "POST":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/undelete")
		return handleUndeleteImage(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "DELETE":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleDeleteImage(imageID, headers)
	// Project routes
	case path == "/api/projects" && method == "GET":
		return handleListProjects(request, headers)
	case path == "/api/projects" && method == "POST":
		return handleCreateProject(request, headers)
	case strings.HasPrefix(path, "/api/projects/") && !strings.Contains(path[len("/api/projects/"):], "/") && method == "PUT":
		projectID := strings.TrimPrefix(path, "/api/projects/")
		return handleUpdateProject(projectID, request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "POST":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleAddToProject(projectID, request, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/images") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/images")
		return handleGetProjectImages(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/generate-zip") && method == "POST":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/generate-zip")
		return handleGenerateZip(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/zips/") && strings.HasSuffix(path, "/download") && method == "GET":
		// Extract projectID and zipKey from path: /api/projects/{projectId}/zips/{zipKey}/download
		parts := strings.Split(path, "/")
		if len(parts) >= 7 {
			projectID := parts[3]
			zipKeyEncoded := strings.Join(parts[5:len(parts)-1], "/")
			// URL-decode the zipKey since it may contain encoded slashes
			zipKey, err := url.QueryUnescape(zipKeyEncoded)
			if err != nil {
				zipKey = zipKeyEncoded // Fall back to encoded version if decode fails
			}
			return handleGetZipDownload(projectID, zipKey, headers)
		}
		return errorResponse(400, "Invalid zip download path", headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/zip-logs") && method == "GET":
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/zip-logs")
		return handleGetZipLogs(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.HasSuffix(path, "/zips") && method == "DELETE":
		// Delete all zips: /api/projects/{projectId}/zips
		projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/projects/"), "/zips")
		return handleDeleteAllZips(projectID, headers)
	case strings.HasPrefix(path, "/api/projects/") && strings.Contains(path, "/zips/") && method == "DELETE":
		// Extract projectID and zipKey from path: /api/projects/{projectId}/zips/{zipKey}
		parts := strings.Split(path, "/")
		if len(parts) >= 6 {
			projectID := parts[3]
			zipKeyEncoded := strings.Join(parts[5:], "/")
			zipKey, err := url.QueryUnescape(zipKeyEncoded)
			if err != nil {
				zipKey = zipKeyEncoded
			}
			return handleDeleteZip(projectID, zipKey, headers)
		}
		return errorResponse(400, "Invalid zip delete path", headers)
	case strings.HasPrefix(path, "/api/projects/") && !strings.Contains(path[len("/api/projects/"):], "/") && method == "DELETE":
		// Delete project: /api/projects/{projectId}
		projectID := strings.TrimPrefix(path, "/api/projects/")
		return handleDeleteProject(projectID, headers)
	// Logs route
	case path == "/api/logs" && method == "GET":
		return handleGetLogs(request.QueryStringParameters, headers)
	default:
		return errorResponse(404, "Not found", headers)
	}
}

func handleLogin(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var loginReq LoginRequest
	if err := json.Unmarshal([]byte(request.Body), &loginReq); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get user from DynamoDB
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(usersTable),
		Key: map[string]*dynamodb.AttributeValue{
			"Username": {S: aws.String(loginReq.Username)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(401, "Invalid credentials", headers)
	}

	var passwordHash string
	if result.Item["PasswordHash"] != nil {
		passwordHash = *result.Item["PasswordHash"].S
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(loginReq.Password)); err != nil {
		return errorResponse(401, "Invalid credentials", headers)
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": loginReq.Username,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return errorResponse(500, "Failed to generate token", headers)
	}

	response := LoginResponse{Token: tokenString}
	body, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// queryAllPages executes a DynamoDB query and handles pagination to return all results
func queryAllPages(input *dynamodb.QueryInput) ([]map[string]*dynamodb.AttributeValue, error) {
	var allItems []map[string]*dynamodb.AttributeValue

	for {
		result, err := ddbClient.Query(input)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, result.Items...)

		// Check if there are more pages
		if result.LastEvaluatedKey == nil {
			break
		}
		// Set the start key for the next page
		input.ExclusiveStartKey = result.LastEvaluatedKey
	}

	return allItems, nil
}

func handleListImages(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get filter parameters from query string
	stateFilter := request.QueryStringParameters["state"]
	groupFilter := request.QueryStringParameters["group"]

	// Default to unreviewed if no state specified
	if stateFilter == "" {
		stateFilter = "unreviewed"
	}

	var allItems []map[string]*dynamodb.AttributeValue
	var err error

	// Determine query based on state filter using StatusIndex
	switch stateFilter {
	case "unreviewed":
		// Query for inbox images using StatusIndex with pagination
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("StatusIndex"),
			KeyConditionExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("inbox")},
			},
		}
		allItems, err = queryAllPages(input)
	case "approved":
		// Query for approved images using StatusIndex with pagination
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("StatusIndex"),
			KeyConditionExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("approved")},
			},
		}
		// Add FilterExpression for group if specified
		if groupFilter != "" && groupFilter != "all" {
			groupNum := 0
			fmt.Sscanf(groupFilter, "%d", &groupNum)
			input.FilterExpression = aws.String("GroupNumber = :group")
			input.ExpressionAttributeValues[":group"] = &dynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", groupNum))}
		}
		allItems, err = queryAllPages(input)
	case "rejected":
		// Query for rejected images using StatusIndex with pagination
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("StatusIndex"),
			KeyConditionExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("rejected")},
			},
		}
		allItems, err = queryAllPages(input)
	case "deleted":
		// Query for deleted images using StatusIndex with pagination
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("StatusIndex"),
			KeyConditionExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("deleted")},
			},
		}
		allItems, err = queryAllPages(input)
	case "all":
		// Query each status and merge results (excluding deleted) with pagination
		statuses := []string{"inbox", "approved", "rejected", "project"}
		for _, status := range statuses {
			items, queryErr := queryAllPages(&dynamodb.QueryInput{
				TableName:              aws.String(imageTable),
				IndexName:              aws.String("StatusIndex"),
				KeyConditionExpression: aws.String("#status = :status"),
				ExpressionAttributeNames: map[string]*string{
					"#status": aws.String("Status"),
				},
				ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
					":status": {S: aws.String(status)},
				},
			})
			if queryErr != nil {
				err = queryErr
				break
			}
			allItems = append(allItems, items...)
		}
	default:
		return errorResponse(400, "Invalid state filter", headers)
	}

	if err != nil {
		fmt.Printf("Error querying images: %v\n", err)
		return errorResponse(500, "Failed to list images", headers)
	}

	// Use a map to deduplicate by OriginalFile, keeping the most recent entry
	seenFiles := make(map[string]ImageResponse)
	for _, item := range allItems {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		// Skip images that are already in a project (unless querying specifically for projects)
		if img.ProjectID != "" && stateFilter != "all" {
			continue
		}

		// Deduplicate by OriginalFile - keep the most recently updated entry
		if existing, found := seenFiles[img.OriginalFile]; found {
			// Compare timestamps - keep the newer one
			existingTime, _ := time.Parse(time.RFC3339, existing.UpdatedDateTime)
			if existing.UpdatedDateTime == "" {
				existingTime, _ = time.Parse(time.RFC3339, existing.InsertedDateTime)
			}
			newTime, _ := time.Parse(time.RFC3339, img.UpdatedDateTime)
			if img.UpdatedDateTime == "" {
				newTime, _ = time.Parse(time.RFC3339, img.InsertedDateTime)
			}
			if newTime.After(existingTime) {
				seenFiles[img.OriginalFile] = img
			}
		} else {
			seenFiles[img.OriginalFile] = img
		}
	}

	// Convert map to slice, stripping heavy fields to reduce payload size
	// Keep: DateTimeOriginal (for date grouping), Keywords (for search)
	// Strip: Description, RelatedFiles, verbose EXIF fields
	images := make([]ImageResponse, 0, len(seenFiles))
	for _, img := range seenFiles {
		// Keep only essential EXIF fields for date grouping
		if img.EXIFData != nil {
			essentialExif := make(map[string]string)
			if v, ok := img.EXIFData["DateTimeOriginal"]; ok {
				essentialExif["DateTimeOriginal"] = v
			}
			if v, ok := img.EXIFData["DateTime"]; ok {
				essentialExif["DateTime"] = v
			}
			img.EXIFData = essentialExif
		}
		// Clear heavy fields to keep response under Lambda's 6MB limit
		img.Description = ""
		img.RelatedFiles = nil
		images = append(images, img)
	}

	body, _ := json.Marshal(images)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleUpdateImage(imageID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var updateReq UpdateImageRequest
	if err := json.Unmarshal([]byte(request.Body), &updateReq); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get current image metadata to check if this is a new review
	getResult, err := withRetry(func() (*dynamodb.GetItemOutput, error) {
		return ddbClient.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(imageID)},
			},
		})
	})
	if err != nil || getResult.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(getResult.Item, &img)

	// Check if this is a new review (moving from unreviewed to reviewed)
	var triggerMove bool
	var destPrefix string
	var newStatus string

	if updateReq.Reviewed == "true" && img.Reviewed == "false" {
		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)

		if updateReq.GroupNumber > 0 {
			// Approved with a group - move to approved/<color>/YYYY/MM/DD
			colorName := getColorName(updateReq.GroupNumber)
			destPrefix = fmt.Sprintf("approved/%s/%s", colorName, datePath)
			newStatus = "approved"
			triggerMove = true
		} else {
			// Rejected (group 0) - move to rejected/YYYY/MM/DD
			destPrefix = "rejected/" + datePath
			newStatus = "rejected"
			triggerMove = true
		}
	}

	// Build update expression
	updateExpr := "SET GroupNumber = :group, UpdatedDateTime = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":group":   {N: aws.String(fmt.Sprintf("%d", updateReq.GroupNumber))},
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}
	exprAttrNames := make(map[string]*string)

	if updateReq.ColorCode != "" {
		updateExpr += ", ColorCode = :color"
		exprAttrValues[":color"] = &dynamodb.AttributeValue{S: aws.String(updateReq.ColorCode)}
	}

	if updateReq.Promoted {
		updateExpr += ", Promoted = :promoted"
		exprAttrValues[":promoted"] = &dynamodb.AttributeValue{BOOL: aws.Bool(true)}
	}

	// Rating is 0-5, where 0 means no rating (only update if explicitly provided)
	if updateReq.Rating != nil && *updateReq.Rating >= 0 && *updateReq.Rating <= 5 {
		updateExpr += ", Rating = :rating"
		exprAttrValues[":rating"] = &dynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", *updateReq.Rating))}
	}

	// Update keywords if provided
	if updateReq.Keywords != nil {
		if len(updateReq.Keywords) > 0 {
			keywordsList := make([]*dynamodb.AttributeValue, len(updateReq.Keywords))
			for i, kw := range updateReq.Keywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			updateExpr += ", Keywords = :keywords"
			exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		} else {
			// Empty array means remove keywords
			updateExpr += " REMOVE Keywords"
		}
	}

	if updateReq.Reviewed != "" {
		updateExpr += ", Reviewed = :reviewed"
		exprAttrValues[":reviewed"] = &dynamodb.AttributeValue{S: aws.String(updateReq.Reviewed)}
	}

	// Set move status to pending if we need to trigger async move
	if triggerMove {
		updateExpr += ", MoveStatus = :moveStatus"
		exprAttrValues[":moveStatus"] = &dynamodb.AttributeValue{S: aws.String("pending")}
	}

	// Update the image metadata
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	}
	if len(exprAttrNames) > 0 {
		updateInput.ExpressionAttributeNames = exprAttrNames
	}

	err = withRetryNoResult(func() error {
		_, updateErr := ddbClient.UpdateItem(updateInput)
		return updateErr
	})
	if err != nil {
		fmt.Printf("Error updating image: %v\n", err)
		return errorResponse(500, "Failed to update image", headers)
	}

	// Store review decision
	reviewID := fmt.Sprintf("review_%d", time.Now().Unix())
	reviewItem := map[string]*dynamodb.AttributeValue{
		"ReviewID":    {S: aws.String(reviewID)},
		"ImageGUID":   {S: aws.String(imageID)},
		"GroupNumber": {N: aws.String(fmt.Sprintf("%d", updateReq.GroupNumber))},
		"ColorCode":   {S: aws.String(updateReq.ColorCode)},
		"Promoted":    {BOOL: aws.Bool(updateReq.Promoted)},
		"Timestamp":   {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	withRetryNoResult(func() error {
		_, putErr := ddbClient.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(reviewGroupsTable),
			Item:      reviewItem,
		})
		return putErr
	})

	// Trigger async file move if needed
	if triggerMove {
		if err := triggerAsyncMove(imageID, destPrefix, newStatus, bucketName); err != nil {
			fmt.Printf("Error triggering async move: %v\n", err)
			// Don't fail the request - the move status will show the issue
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

func handleRegenerateAI(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Check if OpenAI API key is configured
	if openaiAPIKey == "" {
		return errorResponse(400, "AI content generation is not configured", headers)
	}

	// Get image metadata
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Call GPT-4o to analyze the image
	aiResult, err := analyzeImageWithGPT4o(img.Bucket, img.Thumbnail400)
	if err != nil {
		fmt.Printf("Error analyzing image with GPT-4o: %v\n", err)
		return errorResponse(500, fmt.Sprintf("Failed to analyze image: %v", err), headers)
	}

	// Merge AI keywords with existing user keywords (case-insensitive deduplication)
	existingKeywordsLower := make(map[string]bool)
	for _, kw := range img.Keywords {
		existingKeywordsLower[strings.ToLower(kw)] = true
	}

	mergedKeywords := append([]string{}, img.Keywords...) // Start with existing
	for _, kw := range aiResult.Keywords {
		if !existingKeywordsLower[strings.ToLower(kw)] {
			mergedKeywords = append(mergedKeywords, kw)
			existingKeywordsLower[strings.ToLower(kw)] = true
		}
	}

	// Build update expression for keywords and description
	updateExpr := "SET UpdatedDateTime = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	if len(mergedKeywords) > 0 {
		keywordsList := make([]*dynamodb.AttributeValue, len(mergedKeywords))
		for i, kw := range mergedKeywords {
			keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
		}
		updateExpr += ", Keywords = :keywords"
		exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
	}

	if aiResult.Description != "" {
		updateExpr += ", Description = :description"
		exprAttrValues[":description"] = &dynamodb.AttributeValue{S: aws.String(aiResult.Description)}
	}

	// Update the image in DynamoDB
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	})
	if err != nil {
		fmt.Printf("Error updating image with AI content: %v\n", err)
		return errorResponse(500, "Failed to save AI content", headers)
	}

	// Return the updated content
	response := map[string]interface{}{
		"success":     true,
		"keywords":    mergedKeywords,
		"description": aiResult.Description,
	}
	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDownload(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Generate presigned URL
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(img.OriginalFile),
	})

	url, err := req.Presign(15 * time.Minute)
	if err != nil {
		return errorResponse(500, "Failed to generate download URL", headers)
	}

	body, _ := json.Marshal(map[string]string{"url": url})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDeleteImage(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata first
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Check if image is already deleted
	if img.Status == "deleted" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       `{"success": true, "message": "Image already deleted"}`,
		}, nil
	}

	// Get date from EXIF for folder structure
	imageDate := getImageDate(img)
	datePath := buildDatePath(imageDate)
	destPrefix := "deleted/" + datePath

	// Move all files to deleted folder
	newPaths, err := moveImageFiles(bucketName, img, destPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		// If the source file is missing from S3, delete the image record from DynamoDB
		if errors.Is(err, ErrSourceFileMissing) {
			fmt.Printf("Source file missing for image %s, deleting from database\n", imageID)
			if delErr := deleteImageFromDB(imageID); delErr != nil {
				fmt.Printf("Error deleting image from DB: %v\n", delErr)
				return errorResponse(500, "Failed to delete orphaned image record", headers)
			}
			return events.APIGatewayProxyResponse{
				StatusCode: 200,
				Headers:    headers,
				Body:       `{"success": true, "deleted": true, "reason": "source file missing"}`,
			}, nil
		}
		return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
	}

	// Update DynamoDB with new paths and status (instead of deleting)
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status, UpdatedDateTime = :updated"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":    {S: aws.String(newPaths["original"])},
			":t50":     {S: aws.String(newPaths["thumbnail50"])},
			":t400":    {S: aws.String(newPaths["thumbnail400"])},
			":status":  {S: aws.String("deleted")},
			":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
		},
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
	})

	if err != nil {
		fmt.Printf("Error updating metadata: %v\n", err)
		return errorResponse(500, "Failed to update metadata", headers)
	}

	// Decrement project ImageCount if image was in a project
	if img.ProjectID != "" {
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(projectsTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ProjectID": {S: aws.String(img.ProjectID)},
			},
			UpdateExpression: aws.String("ADD ImageCount :dec"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":dec": {N: aws.String("-1")},
			},
		})
		if err != nil {
			fmt.Printf("Warning: Failed to decrement project ImageCount: %v\n", err)
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

func handleUndeleteImage(imageID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get image metadata first
	result, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	if err != nil || result.Item == nil {
		return errorResponse(404, "Image not found", headers)
	}

	var img ImageResponse
	dynamodbattribute.UnmarshalMap(result.Item, &img)

	// Verify image is deleted
	if img.Status != "deleted" {
		return errorResponse(400, "Image is not deleted", headers)
	}

	// Get date from EXIF for folder structure
	imageDate := getImageDate(img)
	datePath := buildDatePath(imageDate)
	destPrefix := "inbox/" + datePath

	// Move all files back to inbox folder
	newPaths, err := moveImageFiles(bucketName, img, destPrefix)
	if err != nil {
		fmt.Printf("Error moving files: %v\n", err)
		// If the source file is missing from S3, delete the image record from DynamoDB
		if errors.Is(err, ErrSourceFileMissing) {
			fmt.Printf("Source file missing for image %s, deleting from database\n", imageID)
			if delErr := deleteImageFromDB(imageID); delErr != nil {
				fmt.Printf("Error deleting image from DB: %v\n", delErr)
				return errorResponse(500, "Failed to delete orphaned image record", headers)
			}
			return events.APIGatewayProxyResponse{
				StatusCode: 200,
				Headers:    headers,
				Body:       `{"success": true, "deleted": true, "reason": "source file missing"}`,
			}, nil
		}
		return errorResponse(500, fmt.Sprintf("Failed to move files: %v", err), headers)
	}

	// Update DynamoDB with new paths and reset status
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression: aws.String("SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, Reviewed = :reviewed, UpdatedDateTime = :updated REMOVE #status"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":orig":     {S: aws.String(newPaths["original"])},
			":t50":      {S: aws.String(newPaths["thumbnail50"])},
			":t400":     {S: aws.String(newPaths["thumbnail400"])},
			":reviewed": {S: aws.String("false")},
			":updated":  {S: aws.String(time.Now().Format(time.RFC3339))},
		},
		ExpressionAttributeNames: map[string]*string{
			"#status": aws.String("Status"),
		},
	})

	if err != nil {
		fmt.Printf("Error updating metadata: %v\n", err)
		return errorResponse(500, "Failed to update metadata", headers)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
	}, nil
}

// Project handlers

func handleListProjects(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Check if we should include archived projects
	includeArchived := request.QueryStringParameters["includeArchived"] == "true"

	result, err := ddbClient.Scan(&dynamodb.ScanInput{
		TableName: aws.String(projectsTable),
	})
	if err != nil {
		fmt.Printf("Error listing projects: %v\n", err)
		return errorResponse(500, "Failed to list projects", headers)
	}

	projects := make([]Project, 0)
	for _, item := range result.Items {
		var p Project
		dynamodbattribute.UnmarshalMap(item, &p)

		// Skip archived projects unless specifically requested
		if p.Archived && !includeArchived {
			continue
		}

		// Calculate actual image count from images table
		actualCount := getProjectImageCount(p.ProjectID)
		if actualCount != p.ImageCount {
			// Update stored count if it differs from actual
			p.ImageCount = actualCount
			updateProjectImageCount(p.ProjectID, actualCount)
		}

		projects = append(projects, p)
	}

	// Sort projects alphabetically by name (case-insensitive)
	sort.Slice(projects, func(i, j int) bool {
		return strings.ToLower(projects[i].Name) < strings.ToLower(projects[j].Name)
	})

	body, _ := json.Marshal(projects)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// getProjectImageCount counts actual images belonging to a project
func getProjectImageCount(projectID string) int {
	result, err := ddbClient.Query(&dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectIndex"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
		Select: aws.String("COUNT"),
	})
	if err != nil {
		fmt.Printf("Error counting project images: %v\n", err)
		return 0
	}
	return int(*result.Count)
}

// updateProjectImageCount updates the stored ImageCount for a project
func updateProjectImageCount(projectID string, count int) {
	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ImageCount = :count"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":count": {N: aws.String(fmt.Sprintf("%d", count))},
		},
	})
	if err != nil {
		fmt.Printf("Warning: Failed to update ImageCount for project %s: %v\n", projectID, err)
	}
}

func handleCreateProject(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req CreateProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	if req.Name == "" {
		return errorResponse(400, "Project name is required", headers)
	}

	// Generate S3-safe prefix from project name
	s3Prefix := sanitizeS3Name(req.Name)

	project := Project{
		ProjectID:  uuid.New().String(),
		Name:       req.Name,
		S3Prefix:   s3Prefix,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ImageCount: 0,
		Keywords:   req.Keywords,
	}

	av, _ := dynamodbattribute.MarshalMap(project)
	_, err := ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(projectsTable),
		Item:      av,
	})
	if err != nil {
		fmt.Printf("Error creating project: %v\n", err)
		return errorResponse(500, "Failed to create project", headers)
	}

	body, _ := json.Marshal(project)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleUpdateProject(projectID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req UpdateProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get existing project
	getResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || getResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(getResult.Item, &project)

	// Build update expression
	updateExpr := "SET UpdatedAt = :updated"
	exprAttrValues := map[string]*dynamodb.AttributeValue{
		":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
	}

	// Update name if provided
	if req.Name != "" {
		updateExpr += ", #name = :name"
		exprAttrValues[":name"] = &dynamodb.AttributeValue{S: aws.String(req.Name)}
		project.Name = req.Name
	}

	// Update keywords (can be empty array to clear)
	if req.Keywords != nil {
		if len(req.Keywords) > 0 {
			keywordsList := make([]*dynamodb.AttributeValue, len(req.Keywords))
			for i, kw := range req.Keywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			updateExpr += ", Keywords = :keywords"
			exprAttrValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		} else {
			// Remove keywords if empty array
			updateExpr += " REMOVE Keywords"
		}
		project.Keywords = req.Keywords
	}

	// Update archived status if provided
	if req.Archived != nil {
		updateExpr += ", Archived = :archived"
		exprAttrValues[":archived"] = &dynamodb.AttributeValue{BOOL: aws.Bool(*req.Archived)}
		project.Archived = *req.Archived
	}

	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
	}

	// Add expression attribute names if we're updating name
	if req.Name != "" {
		updateInput.ExpressionAttributeNames = map[string]*string{
			"#name": aws.String("Name"),
		}
	}

	_, err = ddbClient.UpdateItem(updateInput)
	if err != nil {
		fmt.Printf("Error updating project: %v\n", err)
		return errorResponse(500, "Failed to update project", headers)
	}

	body, _ := json.Marshal(project)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleAddToProject(projectID string, request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	var req AddToProjectRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		return errorResponse(400, "Invalid request body", headers)
	}

	// Get project to verify it exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	var imagesToProcess []map[string]*dynamodb.AttributeValue

	// If a specific imageGUID is provided, add just that image
	if req.ImageGUID != "" {
		imgResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(req.ImageGUID)},
			},
		})
		if err != nil || imgResult.Item == nil {
			return errorResponse(404, "Image not found", headers)
		}
		imagesToProcess = append(imagesToProcess, imgResult.Item)
	} else {
		// Query approved images (Status = 'approved')
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("StatusIndex"),
			KeyConditionExpression: aws.String("#status = :status"),
			ExpressionAttributeNames: map[string]*string{
				"#status": aws.String("Status"),
			},
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":status": {S: aws.String("approved")},
			},
		}

		// Add group filter if not "all"
		if !req.All && req.Group > 0 {
			queryInput.FilterExpression = aws.String("GroupNumber = :group")
			queryInput.ExpressionAttributeValues[":group"] = &dynamodb.AttributeValue{
				N: aws.String(fmt.Sprintf("%d", req.Group)),
			}
		}

		result, err := ddbClient.Query(queryInput)
		if err != nil {
			fmt.Printf("Error querying approved images: %v\n", err)
			return errorResponse(500, "Failed to query images", headers)
		}
		imagesToProcess = result.Items
	}

	// Move each image to project folder using sanitized S3 prefix
	s3Prefix := getProjectS3Prefix(project)
	movedCount := 0
	for _, item := range imagesToProcess {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		imageDate := getImageDate(img)
		datePath := buildDatePath(imageDate)
		destPrefix := fmt.Sprintf("projects/%s/%s", s3Prefix, datePath)

		newPaths, err := moveImageFiles(bucketName, img, destPrefix)
		if err != nil {
			fmt.Printf("Failed to move image %s: %v\n", img.ImageGUID, err)
			// If the source file is missing from S3, delete the image record from DynamoDB
			if errors.Is(err, ErrSourceFileMissing) {
				fmt.Printf("Source file missing for image %s, deleting from database\n", img.ImageGUID)
				deleteImageFromDB(img.ImageGUID)
			}
			continue
		}

		// Build list of raw files for DynamoDB
		var rawFilesList []*dynamodb.AttributeValue
		if rawFilesStr := newPaths["rawFiles"]; rawFilesStr != "" {
			for _, rf := range strings.Split(rawFilesStr, ",") {
				if rf != "" {
					rawFilesList = append(rawFilesList, &dynamodb.AttributeValue{S: aws.String(rf)})
				}
			}
		}

		// Generate AI keywords and description if not already present
		var aiKeywords []string
		var aiDescription string
		if img.Description == "" && openaiAPIKey != "" {
			fmt.Printf("Generating AI analysis for image %s (added to project)\n", img.ImageGUID)
			aiResult, err := analyzeImageWithGPT4o(bucketName, newPaths["thumbnail400"])
			if err != nil {
				fmt.Printf("AI analysis failed for image %s: %v\n", img.ImageGUID, err)
			} else {
				// Merge AI keywords with existing user keywords (case-insensitive deduplication)
				existingKeywordsLower := make(map[string]bool)
				for _, kw := range img.Keywords {
					existingKeywordsLower[strings.ToLower(kw)] = true
				}
				aiKeywords = append([]string{}, img.Keywords...)
				for _, kw := range aiResult.Keywords {
					if !existingKeywordsLower[strings.ToLower(kw)] {
						aiKeywords = append(aiKeywords, kw)
						existingKeywordsLower[strings.ToLower(kw)] = true
					}
				}
				aiDescription = aiResult.Description
				fmt.Printf("AI analysis complete for image %s: %d keywords\n", img.ImageGUID, len(aiKeywords))
			}
		}

		// Update image record
		updateExpr := "SET OriginalFile = :orig, Thumbnail50 = :t50, Thumbnail400 = :t400, #status = :status, ProjectID = :proj, UpdatedDateTime = :updated"
		exprValues := map[string]*dynamodb.AttributeValue{
			":orig":    {S: aws.String(newPaths["original"])},
			":t50":     {S: aws.String(newPaths["thumbnail50"])},
			":t400":    {S: aws.String(newPaths["thumbnail400"])},
			":status":  {S: aws.String("project")},
			":proj":    {S: aws.String(projectID)},
			":updated": {S: aws.String(time.Now().Format(time.RFC3339))},
		}

		// Add RelatedFiles if we found any RAW files
		if len(rawFilesList) > 0 {
			updateExpr += ", RelatedFiles = :rawFiles"
			exprValues[":rawFiles"] = &dynamodb.AttributeValue{L: rawFilesList}
		}

		// Add AI-generated description if available
		if aiDescription != "" {
			updateExpr += ", #desc = :desc"
			exprValues[":desc"] = &dynamodb.AttributeValue{S: aws.String(aiDescription)}
		}

		// Add merged keywords if available
		if len(aiKeywords) > 0 {
			keywordsList := make([]*dynamodb.AttributeValue, len(aiKeywords))
			for i, kw := range aiKeywords {
				keywordsList[i] = &dynamodb.AttributeValue{S: aws.String(kw)}
			}
			updateExpr += ", Keywords = :keywords"
			exprValues[":keywords"] = &dynamodb.AttributeValue{L: keywordsList}
		}

		exprNames := map[string]*string{
			"#status": aws.String("Status"),
		}
		if aiDescription != "" {
			exprNames["#desc"] = aws.String("Description")
		}

		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(imageTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ImageGUID": {S: aws.String(img.ImageGUID)},
			},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeValues: exprValues,
			ExpressionAttributeNames:  exprNames,
		})
		if err != nil {
			fmt.Printf("Failed to update image record %s: %v\n", img.ImageGUID, err)
			continue
		}
		movedCount++
	}

	// Update project image count
	ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("ADD ImageCount :count"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":count": {N: aws.String(fmt.Sprintf("%d", movedCount))},
		},
	})

	body, _ := json.Marshal(map[string]int{"movedCount": movedCount})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetProjectImages(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	result, err := ddbClient.Query(&dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectIndex"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
	})
	if err != nil {
		fmt.Printf("Error querying project images: %v\n", err)
		return errorResponse(500, "Failed to query images", headers)
	}

	images := make([]ImageResponse, 0)
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)
		images = append(images, img)
	}

	body, _ := json.Marshal(images)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGenerateZip(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	if zipLambdaName == "" {
		return errorResponse(500, "Zip generation not configured", headers)
	}

	// Get project to verify it exists and has images
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	if project.ImageCount == 0 {
		return errorResponse(400, "Project has no images to zip", headers)
	}

	// Mark as generating by adding a placeholder zip entry
	placeholderZip := ZipFile{
		Key:        "generating",
		Size:       0,
		ImageCount: project.ImageCount,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Status:     "generating",
	}

	zipFilesList, _ := dynamodbattribute.MarshalList([]ZipFile{placeholderZip})
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ZipFiles = :zips"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":zips": {L: zipFilesList},
		},
	})
	if err != nil {
		fmt.Printf("Error setting zip status: %v\n", err)
	}

	// Invoke the zip Lambda asynchronously
	payload, _ := json.Marshal(map[string]string{"projectId": projectID})

	_, err = lambdaClient.Invoke(&lambdasvc.InvokeInput{
		FunctionName:   aws.String(zipLambdaName),
		InvocationType: aws.String("Event"), // Async invocation
		Payload:        payload,
	})
	if err != nil {
		fmt.Printf("Error invoking zip lambda: %v\n", err)
		return errorResponse(500, "Failed to start zip generation", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "Zip generation started",
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetZipDownload(projectID string, zipKey string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to verify the zip exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Find the zip file in the project's zip files list
	var targetZip *ZipFile
	for _, zf := range project.ZipFiles {
		// The zipKey in the request might be URL-encoded, so we need to match the base filename
		if strings.HasSuffix(zf.Key, zipKey) || zf.Key == zipKey {
			targetZip = &zf
			break
		}
	}

	if targetZip == nil || targetZip.Status != "complete" {
		return errorResponse(404, "Zip file not found or not ready", headers)
	}

	// Generate presigned URL for download
	filename := targetZip.Key[strings.LastIndex(targetZip.Key, "/")+1:]
	req, _ := s3Client.GetObjectRequest(&s3.GetObjectInput{
		Bucket:                     aws.String(bucketName),
		Key:                        aws.String(targetZip.Key),
		ResponseContentDisposition: aws.String(fmt.Sprintf("attachment; filename=\"%s\"", filename)),
	})

	url, err := req.Presign(60 * time.Minute) // 1 hour for large downloads
	if err != nil {
		return errorResponse(500, "Failed to generate download URL", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"url":      url,
		"filename": filename,
		"size":     targetZip.Size,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleGetZipLogs(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to verify it exists
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Check if there's a generating zip
	var generatingZip *ZipFile
	for i, zf := range project.ZipFiles {
		if zf.Status == "generating" {
			generatingZip = &project.ZipFiles[i]
			break
		}
	}

	if generatingZip == nil {
		body, _ := json.Marshal(map[string]interface{}{
			"status":  "no_generation",
			"message": "No zip generation in progress",
		})
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       string(body),
		}, nil
	}

	// Parse the createdAt timestamp
	createdAt, err := time.Parse(time.RFC3339, generatingZip.CreatedAt)
	if err != nil {
		return errorResponse(500, "Failed to parse generation start time", headers)
	}

	// Check if 15 minutes have passed
	elapsed := time.Since(createdAt)
	if elapsed < 15*time.Minute {
		body, _ := json.Marshal(map[string]interface{}{
			"status":      "generating",
			"message":     "Zip generation in progress",
			"elapsedMins": int(elapsed.Minutes()),
		})
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       string(body),
		}, nil
	}

	// 15 minutes have passed, search CloudWatch logs for errors
	logGroupName := "/aws/lambda/ProjectZipGenerator"
	startTime := createdAt.Add(-1 * time.Minute).UnixMilli()
	endTime := createdAt.Add(20 * time.Minute).UnixMilli()

	// Search for error messages in logs
	filterInput := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  aws.String(logGroupName),
		StartTime:     aws.Int64(startTime),
		EndTime:       aws.Int64(endTime),
		FilterPattern: aws.String("ERROR"),
		Limit:         aws.Int64(50),
	}

	var errorMessages []string
	filterResult, err := cwLogsClient.FilterLogEvents(filterInput)
	if err != nil {
		fmt.Printf("Failed to query CloudWatch logs: %v\n", err)
		// Still mark as failed even if we can't get logs
		errorMessages = append(errorMessages, "Zip generation timed out. Unable to retrieve error logs.")
	} else {
		for _, event := range filterResult.Events {
			if event.Message != nil {
				errorMessages = append(errorMessages, *event.Message)
			}
		}
	}

	if len(errorMessages) == 0 {
		errorMessages = append(errorMessages, "Zip generation timed out with no error logs found.")
	}

	// Update the project to mark the zip as failed
	var updatedZipFiles []ZipFile
	for _, zf := range project.ZipFiles {
		if zf.Status == "generating" {
			zf.Status = "failed"
		}
		updatedZipFiles = append(updatedZipFiles, zf)
	}

	zipFilesList, _ := dynamodbattribute.MarshalList(updatedZipFiles)
	ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("SET ZipFiles = :zips"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":zips": {L: zipFilesList},
		},
	})

	body, _ := json.Marshal(map[string]interface{}{
		"status":        "failed",
		"message":       "Zip generation failed after timeout",
		"errorMessages": errorMessages,
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDeleteZip(projectID string, zipKey string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to verify it exists and find the zip
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Find and remove the zip from the list
	var updatedZipFiles []ZipFile
	var foundZip *ZipFile
	for _, zf := range project.ZipFiles {
		if zf.Key == zipKey {
			foundZip = &zf
		} else {
			updatedZipFiles = append(updatedZipFiles, zf)
		}
	}

	if foundZip == nil {
		return errorResponse(404, "Zip file not found", headers)
	}

	// Delete the file from S3
	_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(zipKey),
	})
	if err != nil {
		fmt.Printf("Warning: Failed to delete zip from S3: %v\n", err)
		// Continue anyway to remove from database
	}

	// Update project record to remove the zip from the list
	if len(updatedZipFiles) > 0 {
		zipFilesList, _ := dynamodbattribute.MarshalList(updatedZipFiles)
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(projectsTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ProjectID": {S: aws.String(projectID)},
			},
			UpdateExpression: aws.String("SET ZipFiles = :zips"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":zips": {L: zipFilesList},
			},
		})
	} else {
		// Remove ZipFiles attribute entirely if no zips left
		_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
			TableName: aws.String(projectsTable),
			Key: map[string]*dynamodb.AttributeValue{
				"ProjectID": {S: aws.String(projectID)},
			},
			UpdateExpression: aws.String("REMOVE ZipFiles"),
		})
	}

	if err != nil {
		return errorResponse(500, "Failed to update project", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "Zip file deleted",
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDeleteAllZips(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to find all zips
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Delete all zip files from S3
	for _, zf := range project.ZipFiles {
		_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(zf.Key),
		})
		if err != nil {
			fmt.Printf("Warning: Failed to delete zip %s from S3: %v\n", zf.Key, err)
		}
	}

	// Remove ZipFiles attribute from project
	_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
		UpdateExpression: aws.String("REMOVE ZipFiles"),
	})

	if err != nil {
		return errorResponse(500, "Failed to update project", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "All zip files deleted",
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func handleDeleteProject(projectID string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get project to find all associated data
	projResult, err := ddbClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})
	if err != nil || projResult.Item == nil {
		return errorResponse(404, "Project not found", headers)
	}

	var project Project
	dynamodbattribute.UnmarshalMap(projResult.Item, &project)

	// Delete all zip files from S3
	for _, zf := range project.ZipFiles {
		_, err = s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(zf.Key),
		})
		if err != nil {
			fmt.Printf("Warning: Failed to delete zip %s from S3: %v\n", zf.Key, err)
		}
	}

	// Update all images in this project to remove project association
	// Query images with this projectId
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(imageTable),
		IndexName:              aws.String("ProjectIndex"),
		KeyConditionExpression: aws.String("ProjectID = :pid"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":pid": {S: aws.String(projectID)},
		},
	}

	queryResult, err := ddbClient.Query(queryInput)
	if err == nil {
		for _, item := range queryResult.Items {
			imageGUID := item["ImageGUID"].S
			if imageGUID != nil {
				// Remove project association from image
				_, err = ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
					TableName: aws.String(imageTable),
					Key: map[string]*dynamodb.AttributeValue{
						"ImageGUID": {S: imageGUID},
					},
					UpdateExpression: aws.String("REMOVE ProjectID"),
				})
				if err != nil {
					fmt.Printf("Warning: Failed to update image %s: %v\n", *imageGUID, err)
				}
			}
		}
	}

	// Delete the project record
	_, err = ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ProjectID": {S: aws.String(projectID)},
		},
	})

	if err != nil {
		return errorResponse(500, "Failed to delete project", headers)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"success": true,
		"message": "Project deleted",
	})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func extractToken(headers map[string]string) string {
	auth := headers["Authorization"]
	if auth == "" {
		auth = headers["authorization"]
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func validateToken(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	return err == nil && token.Valid
}

// StatsResponse represents the system health stats
type StatsResponse struct {
	IncomingCount   int    `json:"incomingCount"`
	ProcessedCount  int    `json:"processedCount"`
	UnreviewedCount int    `json:"unreviewedCount"`
	ReviewedCount   int    `json:"reviewedCount"`
	ApprovedCount   int    `json:"approvedCount"`
	RejectedCount   int    `json:"rejectedCount"`
	DeletedCount    int    `json:"deletedCount"`
	SQSQueueDepth   int    `json:"sqsQueueDepth"`
	SQSDLQDepth     int    `json:"sqsDlqDepth"`
	LastUpdated     string `json:"lastUpdated"`
}

func handleGetStats(headers map[string]string) (events.APIGatewayProxyResponse, error) {
	stats := StatsResponse{
		LastUpdated: time.Now().UTC().Format(time.RFC3339),
	}

	// Count incoming items in S3 (objects in incoming/ prefix)
	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String("incoming/"),
	}

	incomingCount := 0
	err := s3Client.ListObjectsV2Pages(listInput, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		incomingCount += len(page.Contents)
		return true
	})
	if err != nil {
		fmt.Printf("Error listing incoming objects: %v\n", err)
	}
	stats.IncomingCount = incomingCount

	// Count images by status from DynamoDB
	// Scan the table and count by status
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(imageTable),
		ProjectionExpression: aws.String("#s, #r"),
		ExpressionAttributeNames: map[string]*string{
			"#s": aws.String("Status"),
			"#r": aws.String("Reviewed"),
		},
	}

	var unreviewedCount, approvedCount, rejectedCount, deletedCount int

	err = ddbClient.ScanPages(scanInput, func(page *dynamodb.ScanOutput, lastPage bool) bool {
		for _, item := range page.Items {
			status := ""
			reviewed := ""
			if item["Status"] != nil && item["Status"].S != nil {
				status = *item["Status"].S
			}
			if item["Reviewed"] != nil && item["Reviewed"].S != nil {
				reviewed = *item["Reviewed"].S
			}

			switch status {
			case "deleted":
				deletedCount++
			case "approved":
				approvedCount++
			case "rejected":
				rejectedCount++
			default:
				// Check if reviewed
				if reviewed == "no" || reviewed == "" {
					unreviewedCount++
				} else if status == "inbox" || status == "" {
					// Reviewed but still in inbox - could be approved or rejected based on groupNumber
					// For simplicity, count as reviewed
					approvedCount++
				}
			}
		}
		return true
	})
	if err != nil {
		fmt.Printf("Error scanning DynamoDB: %v\n", err)
	}

	stats.UnreviewedCount = unreviewedCount
	stats.ApprovedCount = approvedCount
	stats.RejectedCount = rejectedCount
	stats.DeletedCount = deletedCount
	stats.ProcessedCount = unreviewedCount + approvedCount + rejectedCount + deletedCount
	stats.ReviewedCount = approvedCount + rejectedCount + deletedCount

	// Get SQS queue depth
	if sqsQueueURL != "" {
		queueAttrs, err := sqsClient.GetQueueAttributes(&sqs.GetQueueAttributesInput{
			QueueUrl: aws.String(sqsQueueURL),
			AttributeNames: []*string{
				aws.String("ApproximateNumberOfMessages"),
				aws.String("ApproximateNumberOfMessagesNotVisible"),
			},
		})
		if err != nil {
			fmt.Printf("Error getting SQS queue attributes: %v\n", err)
		} else {
			if val, ok := queueAttrs.Attributes["ApproximateNumberOfMessages"]; ok {
				if n, err := strconv.Atoi(*val); err == nil {
					stats.SQSQueueDepth += n
				}
			}
			if val, ok := queueAttrs.Attributes["ApproximateNumberOfMessagesNotVisible"]; ok {
				if n, err := strconv.Atoi(*val); err == nil {
					stats.SQSQueueDepth += n
				}
			}
		}
	}

	// Get DLQ depth
	if sqsDLQURL != "" {
		dlqAttrs, err := sqsClient.GetQueueAttributes(&sqs.GetQueueAttributesInput{
			QueueUrl: aws.String(sqsDLQURL),
			AttributeNames: []*string{
				aws.String("ApproximateNumberOfMessages"),
			},
		})
		if err != nil {
			fmt.Printf("Error getting DLQ attributes: %v\n", err)
		} else {
			if val, ok := dlqAttrs.Attributes["ApproximateNumberOfMessages"]; ok {
				if n, err := strconv.Atoi(*val); err == nil {
					stats.SQSDLQDepth = n
				}
			}
		}
	}

	body, _ := json.Marshal(stats)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func errorResponse(statusCode int, message string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{"error": message})
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// LogsResponse is the response for the logs endpoint
type LogsResponse struct {
	Logs  []LogEntry `json:"logs"`
	Count int        `json:"count"`
}

func handleGetLogs(params map[string]string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Validate required function parameter
	functionName := params["function"]
	if functionName == "" {
		return errorResponse(400, "Missing required parameter: function", headers)
	}

	// Validate function name is one of the allowed values
	allowedFunctions := map[string]string{
		"ImageThumbnailGenerator": "/aws/lambda/ImageThumbnailGenerator",
		"ImageReviewApi":          "/aws/lambda/ImageReviewApi",
		"ProjectZipGenerator":     "/aws/lambda/ProjectZipGenerator",
	}

	logGroupName, ok := allowedFunctions[functionName]
	if !ok {
		return errorResponse(400, "Invalid function name. Allowed values: ImageThumbnailGenerator, ImageReviewApi, ProjectZipGenerator", headers)
	}

	// Parse hours parameter (default 1)
	hours := 1
	if hoursStr, ok := params["hours"]; ok && hoursStr != "" {
		switch hoursStr {
		case "1":
			hours = 1
		case "24":
			hours = 24
		case "48":
			hours = 48
		case "168":
			hours = 168
		default:
			return errorResponse(400, "Invalid hours parameter. Allowed values: 1, 24, 48, 168", headers)
		}
	}

	// Parse filter parameter (default "error")
	filterMode := "error"
	if filter, ok := params["filter"]; ok && filter != "" {
		if filter != "error" && filter != "all" {
			return errorResponse(400, "Invalid filter parameter. Allowed values: error, all", headers)
		}
		filterMode = filter
	}

	// Calculate start time
	startTime := time.Now().Add(-time.Duration(hours) * time.Hour)

	// Build filter pattern for error logs
	var filterPattern *string
	if filterMode == "error" {
		pattern := "?ERROR ?error ?Error ?Exception ?exception ?500 ?403 ?401"
		filterPattern = aws.String(pattern)
	}

	// Query CloudWatch Logs
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(logGroupName),
		StartTime:    aws.Int64(startTime.UnixMilli()),
		Limit:        aws.Int64(500),
	}
	if filterPattern != nil {
		input.FilterPattern = filterPattern
	}

	result, err := cwLogsClient.FilterLogEvents(input)
	if err != nil {
		return errorResponse(500, fmt.Sprintf("Failed to fetch logs: %v", err), headers)
	}

	// Convert to response format
	logs := make([]LogEntry, 0, len(result.Events))
	for _, event := range result.Events {
		timestamp := time.UnixMilli(*event.Timestamp).UTC().Format(time.RFC3339Nano)
		logs = append(logs, LogEntry{
			Timestamp: timestamp,
			Message:   *event.Message,
		})
	}

	response := LogsResponse{
		Logs:  logs,
		Count: len(logs),
	}

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
