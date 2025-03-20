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

// Secrets Manager のレスポンス構造体
type DBSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	DbName   string `json:"dbname"`
}

// リクエストボディ用の構造体
type CreateUserRequest struct {
	Username string `json:"username"`
}

// Lambda のハンドラー関数
func handler(ctx context.Context, event json.RawMessage) (events.APIGatewayProxyResponse, error) {
	var req CreateUserRequest
	if err := json.Unmarshal([]byte(event), &req); err != nil || req.Username == "" {
		// エラーメッセージをBodyに含める
		var errorMessage string
		if err != nil {
			// json.Unmarshal のエラー詳細を Body に含める
			errorMessage += fmt.Sprintf("Error unmarshalling JSON: %v\n", err)
		}
		if req.Username == "" {
			// 'username' が空の場合のエラーメッセージ
			errorMessage += "Error: 'username' field is missing in the request.\n"
		}

		// エラーレスポンスを返す
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       fmt.Sprintf("Invalid request body: %s", errorMessage),
		}, nil
	}

	// Secrets Manager から DB 認証情報を取得
	secretName := os.Getenv("DB_SECRET_ARN")
	region := os.Getenv("AWS_REGION")
	secret, err := getDBSecret(secretName, region)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to get secret: %v", err),
		}, nil
	}

	// ログ出力して接続情報を確認
	fmt.Printf("DB Host: %s, DB Port: %s\n", secret.Host, secret.Port)
	fmt.Printf("DB Username: %s\n", secret.Username)
	fmt.Printf("DB Name: %s\n", secret.DbName)
	fmt.Printf("DB Secret ARN: %s\n", secretName)
	fmt.Printf("Region: %s\n", region)

	// DB に接続
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?multiStatements=true",
		secret.Username, secret.Password, secret.Host, secret.Port, secret.DbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to connect to DB: %v", err),
		}, nil
	}
	defer db.Close()

	// ユーザー作成
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

// Secrets Manager から RDS の認証情報を取得
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
