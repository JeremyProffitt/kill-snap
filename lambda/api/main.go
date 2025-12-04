package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	sess              *session.Session
	ddbClient         *dynamodb.DynamoDB
	s3Client          *s3.S3
	bucketName        string
	imageTable        string
	usersTable        string
	reviewGroupsTable string
	adminUsername     string
	adminPassword     string
	jwtSecret         = []byte("kill-snap-secret-key-change-in-production")
)

func init() {
	sess = session.Must(session.NewSession())
	ddbClient = dynamodb.New(sess)
	s3Client = s3.New(sess)
	bucketName = os.Getenv("BUCKET_NAME")
	imageTable = os.Getenv("IMAGE_TABLE")
	usersTable = os.Getenv("USERS_TABLE")
	reviewGroupsTable = os.Getenv("REVIEW_GROUPS_TABLE")
	adminUsername = os.Getenv("ADMIN_USERNAME")
	adminPassword = os.Getenv("ADMIN_PASSWORD")

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
	OriginalFile     string            `json:"originalFile"`
	Thumbnail50      string            `json:"thumbnail50"`
	Thumbnail400     string            `json:"thumbnail400"`
	Bucket           string            `json:"bucket"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	FileSize         int64             `json:"fileSize"`
	Reviewed         string            `json:"reviewed"`
	GroupNumber      int               `json:"groupNumber,omitempty"`
	ColorCode        string            `json:"colorCode,omitempty"`
	Promoted         bool              `json:"promoted,omitempty"`
	EXIFData         map[string]string `json:"exifData,omitempty"`
	RelatedFiles     []string          `json:"relatedFiles,omitempty"`
	InsertedDateTime string            `json:"insertedDateTime,omitempty"`
	UpdatedDateTime  string            `json:"updatedDateTime,omitempty"`
}

type UpdateImageRequest struct {
	GroupNumber int    `json:"groupNumber,omitempty"`
	ColorCode   string `json:"colorCode,omitempty"`
	Promoted    bool   `json:"promoted,omitempty"`
	Reviewed    string `json:"reviewed,omitempty"`
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

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Content-Type":                "application/json",
		"Access-Control-Allow-Origin": "*",
		"Access-Control-Allow-Headers": "Content-Type,Authorization",
		"Access-Control-Allow-Methods": "GET,POST,PUT,DELETE,OPTIONS",
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
	case path == "/api/images" && method == "GET":
		return handleListImages(request, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "PUT":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleUpdateImage(imageID, request, headers)
	case strings.HasPrefix(path, "/api/images/") && strings.HasSuffix(path, "/download") && method == "GET":
		imageID := strings.TrimSuffix(strings.TrimPrefix(path, "/api/images/"), "/download")
		return handleDownload(imageID, headers)
	case strings.HasPrefix(path, "/api/images/") && method == "DELETE":
		imageID := strings.TrimPrefix(path, "/api/images/")
		return handleDeleteImage(imageID, headers)
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

func handleListImages(request events.APIGatewayProxyRequest, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	// Get filter parameters from query string
	stateFilter := request.QueryStringParameters["state"]
	groupFilter := request.QueryStringParameters["group"]

	// Default to unreviewed if no state specified
	if stateFilter == "" {
		stateFilter = "unreviewed"
	}

	var result *dynamodb.QueryOutput
	var err error

	// Determine reviewed value based on state filter
	switch stateFilter {
	case "unreviewed":
		// Query for unreviewed images using GSI
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("ReviewedIndex"),
			KeyConditionExpression: aws.String("Reviewed = :reviewed"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":reviewed": {S: aws.String("false")},
			},
		}
		result, err = ddbClient.Query(input)
	case "approved", "rejected":
		// Query for reviewed images using GSI
		input := &dynamodb.QueryInput{
			TableName:              aws.String(imageTable),
			IndexName:              aws.String("ReviewedIndex"),
			KeyConditionExpression: aws.String("Reviewed = :reviewed"),
			ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
				":reviewed": {S: aws.String("true")},
			},
		}
		result, err = ddbClient.Query(input)
	case "all":
		// Scan all images
		scanResult, scanErr := ddbClient.Scan(&dynamodb.ScanInput{
			TableName: aws.String(imageTable),
		})
		if scanErr != nil {
			err = scanErr
		} else {
			result = &dynamodb.QueryOutput{Items: scanResult.Items}
		}
	default:
		return errorResponse(400, "Invalid state filter", headers)
	}

	if err != nil {
		fmt.Printf("Error querying images: %v\n", err)
		return errorResponse(500, "Failed to list images", headers)
	}

	// Initialize as empty slice to ensure JSON returns [] instead of null
	images := make([]ImageResponse, 0)
	for _, item := range result.Items {
		var img ImageResponse
		dynamodbattribute.UnmarshalMap(item, &img)

		// Filter by state (approved vs rejected)
		if stateFilter == "approved" && img.GroupNumber == 0 {
			continue // Skip rejected (no group assigned)
		}
		if stateFilter == "rejected" && img.GroupNumber > 0 {
			continue // Skip approved (has group assigned)
		}

		// Filter by group if specified
		if groupFilter != "" && groupFilter != "all" {
			groupNum := 0
			fmt.Sscanf(groupFilter, "%d", &groupNum)
			if img.GroupNumber != groupNum {
				continue
			}
		}

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

	// Build update expression
	updateExpr := "SET "
	exprAttrValues := make(map[string]*dynamodb.AttributeValue)
	first := true

	// Always set GroupNumber (0 means rejected/no group)
	if !first {
		updateExpr += ", "
	}
	updateExpr += "GroupNumber = :group"
	exprAttrValues[":group"] = &dynamodb.AttributeValue{N: aws.String(fmt.Sprintf("%d", updateReq.GroupNumber))}
	first = false

	if updateReq.ColorCode != "" {
		if !first {
			updateExpr += ", "
		}
		updateExpr += "ColorCode = :color"
		exprAttrValues[":color"] = &dynamodb.AttributeValue{S: aws.String(updateReq.ColorCode)}
		first = false
	}

	if updateReq.Promoted {
		if !first {
			updateExpr += ", "
		}
		updateExpr += "Promoted = :promoted"
		exprAttrValues[":promoted"] = &dynamodb.AttributeValue{BOOL: aws.Bool(true)}
		first = false
	}

	if updateReq.Reviewed != "" {
		if !first {
			updateExpr += ", "
		}
		updateExpr += "Reviewed = :reviewed"
		exprAttrValues[":reviewed"] = &dynamodb.AttributeValue{S: aws.String(updateReq.Reviewed)}
		first = false
	}

	// Always update UpdatedDateTime
	if !first {
		updateExpr += ", "
	}
	updateExpr += "UpdatedDateTime = :updated"
	exprAttrValues[":updated"] = &dynamodb.AttributeValue{S: aws.String(time.Now().Format(time.RFC3339))}

	// Update the image metadata
	_, err := ddbClient.UpdateItem(&dynamodb.UpdateItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrValues,
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

	ddbClient.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(reviewGroupsTable),
		Item:      reviewItem,
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
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

	// Delete from S3
	filesToDelete := []string{img.OriginalFile, img.Thumbnail50, img.Thumbnail400}
	for _, file := range filesToDelete {
		s3Client.DeleteObject(&s3.DeleteObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(file),
		})
	}

	// Delete from DynamoDB
	ddbClient.DeleteItem(&dynamodb.DeleteItemInput{
		TableName: aws.String(imageTable),
		Key: map[string]*dynamodb.AttributeValue{
			"ImageGUID": {S: aws.String(imageID)},
		},
	})

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       `{"success": true}`,
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

func errorResponse(statusCode int, message string, headers map[string]string) (events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(map[string]string{"error": message})
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    headers,
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
