package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	_ "github.com/go-sql-driver/mysql"
)

type DBSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
}

func handler(ctx context.Context, event json.RawMessage) (events.APIGatewayProxyResponse, error) {
	var req CreateUserRequest
	if err := json.Unmarshal([]byte(event), &req); err != nil || req.Username == "" {
		var errorMessage string
		if err != nil {
			errorMessage += fmt.Sprintf("Error unmarshalling JSON: %v\n", err)
		}
		if req.Username == "" {
			errorMessage += "Error: 'username' field is missing in the request.\n"
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       fmt.Sprintf("Invalid request body: %s", errorMessage),
		}, nil
	}

	secretName := os.Getenv("DB_SECRET_ARN")
	region := os.Getenv("AWS_REGION")
	secret, err := getDBSecret(secretName, region)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to get secret: %v", err),
		}, nil
	}

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		secret.Username, secret.Password, host, port, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to connect to DB: %v", err),
		}, nil
	}
	defer db.Close()

	if err := createDBUser(db, req.Username); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to create user: %v", err),
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Body:       fmt.Sprintf("User %s created successfully!", req.Username),
	}, nil
}

func getDBSecret(secretName, region string) (*DBSecret, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	svc := secretsmanager.NewFromConfig(cfg)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		return nil, err
	}

	var secret DBSecret
	if err := json.Unmarshal([]byte(*result.SecretString), &secret); err != nil {
		return nil, err
	}

	return &secret, nil
}

// 新しいデータベースユーザーを作成
func createDBUser(db *sql.DB, username string) error {
	// ユーザーを作成
	createUserQuery := fmt.Sprintf("CREATE USER '%s' IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';", username)
	_, err := db.Exec(createUserQuery)
	if err != nil {
		return err
	}

	// ユーザーに SSL 必須を設定
	alterUserQuery := fmt.Sprintf("ALTER USER '%s' REQUIRE SSL;", username)
	_, err = db.Exec(alterUserQuery)
	return err
}

func main() {
	lambda.Start(handler)
}
