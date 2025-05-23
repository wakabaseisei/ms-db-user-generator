# ms-db-user-generator

このリポジトリは、Aurora MySQL に対して IAM 認証で接続するユーザーを Lambda 関数で作成するツールを提供します。  
CIにより Docker イメージが ECR に push され、そのイメージを用いて Lambda 関数が作成され、DB ユーザー作成処理が実行されます。

---

## 🧩 概要

- Secrets Manager に保存されたクレデンシャル情報を使用して Aurora に接続
- `CREATE USER`, `REQUIRE SSL`, `GRANT` を実行して IAM 認証対応の DB ユーザーを作成
- 作成された Docker イメージは、**ms-infra** の `modules/database` から使用されます  
  👉 `locals.tf` の `ms_db_user_generator.image_tag` にて ECR タグを指定

---

## 📁 ディレクトリ構成

```
.
├── .github/
│ └── workflows/ # CI定義（ECRへのbuild & push）
│ └── build-and-push.yml
├── internal/
│ └── cmd/
│ └── gen/
│ └── main.go # Lambda本体（ユーザー作成処理）
├── Dockerfile # Lambdaイメージ用Dockerfile
├── go.mod / go.sum # Go module 定義
└── README.md
```

---

## 🛠 使用例（Lambdaでの実行）

Lambda に設定された環境変数に基づき、Secrets Manager から DB 認証情報を取得し、IAM 認証ユーザーを作成します。

必要な環境変数：

| 変数名             | 説明                                       |
|------------------|------------------------------------------|
| `AWS_REGION`     | AWS リージョン                             |
| `DB_SECRET_ARN`  | Secrets Manager に格納されたクレデンシャルの ARN |
| `DB_HOST`        | Aurora のエンドポイント                      |
| `DB_PORT`        | Aurora のポート（例: 3306）                   |
| `DB_NAME`        | 対象のデータベース名                           |

---

## 🧩 Terraform連携（ms-infraリポジトリ）

本プロジェクトのECRイメージは、[`ms-infra`](https://github.com/wakabaseisei/ms-infra) リポジトリの `modules/database` にて **Aurora MySQLの初期ユーザー作成Lambda** として利用されます。

Terraformで以下のように定義されており：

```
resource "aws_lambda_function" "db_user_generator_lambda" {
  function_name = "db-user-generator-lambda-${var.cluster_identifier}"
  image_uri     = "${data.aws_caller_identity.current.account_id}.dkr.ecr.${data.aws_region.current.name}.amazonaws.com/ms-db-user-generator:${local.ms_db_user_generator.image_tag}"
  ...
}
```

この image_tag は、GitHub Actions によってECRへPushされた最新のタグに差し替えてください：
```
locals {
  ms_db_user_generator = {
    image_tag = "dev-2025XXXX-XXXXXX-<git-sha>"
  }
}
```

> 💡 terraform_data リソースにより、LambdaはTerraform Apply時にInvokeされ、初期DBユーザーが自動作成されます。

---

## 🚀 デプロイ・運用フロー

### 👤 プラットフォーム管理者の作業
1. **main ブランチに push**
2. **GitHub Actions により Docker イメージが ECR に push**
3. **`ms-infra` リポジトリの `modules/database` で `local.ms_db_user_generator.image_tag` を差し替え**

### 👤 マイクロサービスオーナーの作業
4. **`modules/database` を呼び出すマイクロサービス（例: `services/ms-user/dev`）にて Terraform Apply を実行**
5. **Apply により Lambda 関数が生成・実行され、IAM 認証ユーザーが Aurora に作成される**
