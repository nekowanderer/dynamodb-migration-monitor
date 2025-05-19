# DynamoDB Migration Monitor

[English](#english) | [中文](#traditional-chinese)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# English

A monitoring and validation tool for AWS DynamoDB table migration process. It monitors the DynamoDB Stream of the target table to track migration progress and perform sampling validation in real-time.

## Monitoring Methods

Currently supported:
- Stream-based monitoring: Uses DynamoDB Streams to track changes in real-time

Coming soon:
- Table query-based monitoring: Will support direct table comparison through querying

## Migration Architecture

```
Source Table ──────┐
                  │
                  v
            Migration Process
                  │
                  v
Target Table ─── Stream ─── Migration Monitor
```

- Migration Process: Copies data from source table to target table
- Target Table Stream: DynamoDB Stream enabled on target table
- Migration Monitor: Monitors stream and performs validation

## Project Setup

1. Clone the repository:
```bash
git clone https://github.com/yotsuba1022/dynamodb-migration-monitor.git
cd dynamodb-migration-monitor
```

2. Download dependencies:
```bash
go mod download
```

3. Build the project:
```bash
go build
```

## Usage

### Basic Command Template

```bash
./dynamodb-migration-monitor \
  --source-profile <source_profile> \
  --target-profile <target_profile> \
  --stream-arn <stream_arn> \
  --target-table <table_name> \
  --partition-key <pk_name> \
  --region <aws_region>
```

### Complete Example

```bash
./dynamodb-migration-monitor \
  --source-profile source_profile \
  --target-profile target_profile \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:123456789012:table/my-table/stream/2024-01-01T00:00:00.000" \
  --target-table "my-table" \
  --partition-key "user_id" \
  --sort-key "timestamp" \
  --region ap-northeast-1 \
  --sample-rate 50
```

### Parameters

Required:
- `--source-profile`: Source AWS profile name
- `--target-profile`: Target AWS profile name
- `--stream-arn`: Target table's Stream ARN
- `--target-table`: Target table name
- `--partition-key`: Partition key name

Optional:
- `--sort-key`: Sort key name (if table has one)
- `--region`: AWS Region (default: ap-northeast-1)
- `--sample-rate`: Validation sampling rate (default: 100)

## About Sampling Rate and Statistical Confidence

In large-scale data migration (e.g., 30 million records), sampling validation is both effective and reliable. Here's what the sampling rate means:

Using the default sampling rate (100):
- For 30 million records, approximately 300,000 will be validated
- With a 95% confidence level, the margin of error is about ±0.2%
- If sampling validation shows 99.9% accuracy, we can be 95% confident that the overall accuracy is between 99.7% and 100%

This sample size is statistically significant while significantly reducing validation time and resource consumption.

## Monitoring Output

Statistics are displayed every 30 seconds, including:
- Total and unique event counts
- INSERT and MODIFY operation counts
- Average events per second
- Validation success rate

## Prerequisites and Notes

1. AWS CLI Setup:
   - AWS CLI must be installed
   - Source and target AWS Profiles must be configured:
     ```bash
     # Check AWS Profile configuration
     aws configure list --profile <profile_name>
     
     # Verify DynamoDB access
     aws dynamodb list-tables --profile <profile_name>
     ```
   - For SSO login, ensure token is not expired:
     ```bash
     # SSO login
     aws sso login --profile <profile_name>
     ```

2. DynamoDB Stream Configuration:
   - Must be enabled with `NEW_IMAGES`
   - Verify stream configuration:
     ```bash
     aws dynamodb describe-table --table-name <table_name> --profile <profile_name> | grep StreamSpecification -A 5
     ```
   - Enable stream if needed:
     ```bash
     aws dynamodb update-table \
       --table-name <table_name> \
       --stream-specification StreamEnabled=true,StreamViewType=NEW_IMAGES \
       --profile <profile_name>
     ```

3. Required AWS Permissions:
   - Read access to source table
   - Stream read access to target table
   - Minimum required permissions:
     - `dynamodb:GetItem` (source table)
     - `dynamodb:DescribeTable`
     - `dynamodb:DescribeStream`
     - `dynamodb:GetRecords`
     - `dynamodb:GetShardIterator`
     - `dynamodb:ListStreams`

4. It's recommended to test with a small dataset before large-scale migration

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

---

# Traditional Chinese

[回到頂部 ↑](#dynamodb-migration-monitor)

此專案用於監控和驗證 DynamoDB 表格遷移的過程。透過監聽目標表格（Target Table）的 DynamoDB Stream，我們可以即時追蹤資料遷移的狀態並進行抽樣驗證。

## 監控方式

目前支援：
- 基於 Stream 的監控：使用 DynamoDB Streams 即時追蹤資料變更

未來計畫：
- 基於表格查詢的監控：將支援直接比對表格資料的方式

## 遷移架構

```
Source Table (舊表格) ──────┐
                          │
                          v
                    Migration Process
                          │
                          v
Target Table (新表格) ─── Stream ─── Migration Monitor
```

- Migration Process：負責將資料從來源表格複製到目標表格
- Target Table Stream：在目標表格上啟用 DynamoDB Stream
- Migration Monitor：監聽 Stream 並進行驗證

## 專案初始化

1. 下載專案：
```bash
git clone https://github.com/yotsuba1022/dynamodb-migration-monitor.git
cd dynamodb-migration-monitor
```

2. 下載依賴：
```bash
go mod download
```

3. 編譯專案：
```bash
go build
```

## 使用方式

### 基本指令範本

```bash
./dynamodb-migration-monitor \
  --source-profile <source_profile> \
  --target-profile <target_profile> \
  --stream-arn <stream_arn> \
  --target-table <table_name> \
  --partition-key <pk_name> \
  --region <aws_region>
```

### 完整範例

```bash
./dynamodb-migration-monitor \
  --source-profile source_profile \
  --target-profile target_profile \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:123456789012:table/my-table/stream/2024-01-01T00:00:00.000" \
  --target-table "my-table" \
  --partition-key "user_id" \
  --sort-key "timestamp" \
  --region ap-northeast-1 \
  --sample-rate 50
```

### 參數說明

必要參數：
- `--source-profile`：來源 AWS profile 名稱
- `--target-profile`：目標 AWS profile 名稱
- `--stream-arn`：目標表格的 Stream ARN
- `--target-table`：目標表格名稱
- `--partition-key`：分區鍵名稱

選填參數：
- `--sort-key`：排序鍵名稱（如果表格有的話）
- `--region`：AWS Region（預設：ap-northeast-1）
- `--sample-rate`：驗證抽樣率（預設：100）

## 關於抽樣率和統計可信度

在大規模資料遷移中（例如 3000 萬筆資料），使用抽樣驗證是一個有效且可靠的方法。以下說明抽樣率的統計意義：

假設使用預設的抽樣率（100）：
- 在 3000 萬筆資料中，會驗證約 30 萬筆
- 以 95% 信心水準計算，誤差範圍約為 ±0.2%
- 這表示如果抽樣驗證顯示 99.9% 的準確率，我們可以有 95% 的信心說整體資料的準確率在 99.7% 到 100% 之間

這個抽樣規模已經足夠大，能夠提供統計學上顯著的結果，同時又能大幅減少驗證的時間和資源消耗。

## 監控輸出

程式會每 30 秒顯示一次統計資訊，包含：
- 總事件數和唯一事件數
- INSERT 和 MODIFY 操作的數量
- 每秒平均事件數
- 驗證成功率

## 注意事項

1. AWS CLI 設定：
   - 確保已安裝 AWS CLI
   - 已設定好來源和目標的 AWS Profile：
     ```bash
     # 檢查 AWS Profile 設定
     aws configure list --profile <profile_name>
     
     # 確認可以存取 DynamoDB
     aws dynamodb list-tables --profile <profile_name>
     ```
   - 如果使用 SSO 登入，請確保 token 未過期：
     ```bash
     # SSO 登入
     aws sso login --profile <profile_name>
     ```

2. 確保目標表格已啟用 DynamoDB Stream：
   - 需要設定為 `NEW_IMAGES`（只需要新的資料狀態）
   - 可以用以下指令確認：
     ```bash
     aws dynamodb describe-table --table-name <table_name> --profile <profile_name> | grep StreamSpecification -A 5
     ```
   - 如果需要啟用 Stream：
     ```bash
     aws dynamodb update-table \
       --table-name <table_name> \
       --stream-specification StreamEnabled=true,StreamViewType=NEW_IMAGES \
       --profile <profile_name>
     ```

3. AWS Profile 需要有適當的權限：
   - 來源表格的讀取權限
   - 目標表格的 Stream 讀取權限
   - 建議的最小權限：
     - `dynamodb:GetItem`（來源表格）
     - `dynamodb:DescribeTable`
     - `dynamodb:DescribeStream`
     - `dynamodb:GetRecords`
     - `dynamodb:GetShardIterator`
     - `dynamodb:ListStreams`

4. 建議先用小批量資料測試再進行大規模遷移

## 授權條款

本專案使用 Apache License 2.0 授權 - 詳見 [LICENSE](LICENSE) 檔案。
