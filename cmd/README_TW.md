# DynamoDB 測試工具

此目錄包含兩個輔助工具，用於幫助測試 DynamoDB Migration Monitor：

## datagen - 資料產生器

`datagen` 工具可以快速產生 DynamoDB 測試資料，並將所有產生的鍵值對記錄到 CSV 檔案中，以便之後可以刪除這些資料。

### 功能

- 產生指定數量的測試資料
- 可選擇單筆寫入或批次寫入
- 控制寫入間隔，方便測試串流處理
- 將產生的鍵值對記錄到 CSV 檔案中
- 可自訂分區鍵和排序鍵名稱

### 使用方式

```bash
go run ./cmd/datagen/main.go \
  --profile <AWS_設定檔> \
  --table <資料表名稱> \
  --count <資料數量> \
  --partition-key <分區鍵名稱> \
  --sort-key <排序鍵名稱> \
  --region <AWS區域> \
  --wait <寫入間隔毫秒> \
  --output <輸出CSV檔案> \
  --batch
```

### 參數說明

| 參數 | 必填 | 預設值 | 說明 |
|------|------|--------|------|
| `--profile` | 是 | 無 | AWS 設定檔名稱 |
| `--table` | 是 | 無 | DynamoDB 資料表名稱 |
| `--count` | 否 | 10 | 要產生的資料筆數 |
| `--partition-key` | 否 | pk | 分區鍵名稱 |
| `--sort-key` | 否 | sk | 排序鍵名稱 |
| `--region` | 否 | ap-northeast-1 | AWS 區域 |
| `--wait` | 否 | 0 | 資料寫入間隔（毫秒），僅單筆寫入模式有效 |
| `--output` | 否 | 無 | 輸出 CSV 檔案路徑，記錄所有產生的鍵值對 |
| `--batch` | 否 | false | 啟用批次寫入模式 |

### 範例

```bash
# 產生 100 筆測試資料，每筆間隔 100 毫秒
go run ./cmd/datagen/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --count 100 \
  --partition-key pk \
  --sort-key sk \
  --region ap-northeast-1 \
  --wait 100 \
  --output ./test_keys.csv

# 批次寫入 1000 筆資料
go run ./cmd/datagen/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --count 1000 \
  --batch \
  --output ./test_keys.csv
```

---

## datadel - 資料刪除器

`datadel` 工具可以從 CSV 檔案讀取鍵值對，並從 DynamoDB 資料表中刪除對應的資料。

### 功能

- 從 CSV 檔案讀取鍵值對
- 可選擇單筆刪除或批次刪除
- 支援乾跑模式（不實際刪除資料）
- 支援詳細輸出模式
- 可自訂分區鍵和排序鍵名稱

### 使用方式

```bash
go run ./cmd/datadel/main.go \
  --profile <AWS_設定檔> \
  --table <資料表名稱> \
  --input <CSV檔案> \
  --partition-key <分區鍵名稱> \
  --sort-key <排序鍵名稱> \
  --region <AWS區域> \
  --wait <刪除間隔毫秒> \
  --batch \
  --dry-run \
  --verbose
```

### 參數說明

| 參數 | 必填 | 預設值 | 說明 |
|------|------|--------|------|
| `--profile` | 是 | 無 | AWS 設定檔名稱 |
| `--table` | 是 | 無 | DynamoDB 資料表名稱 |
| `--input` | 是 | 無 | CSV 檔案路徑，包含要刪除的鍵值對 |
| `--partition-key` | 否 | pk | 分區鍵名稱 |
| `--sort-key` | 否 | sk | 排序鍵名稱 |
| `--region` | 否 | ap-northeast-1 | AWS 區域 |
| `--wait` | 否 | 0 | 資料刪除間隔（毫秒），僅單筆刪除模式有效 |
| `--batch` | 否 | false | 啟用批次刪除模式 |
| `--dry-run` | 否 | false | 乾跑模式，不實際刪除資料 |
| `--verbose` | 否 | false | 詳細輸出模式 |

### 範例

```bash
# Dry run 模式，查看會刪除哪些資料
go run ./cmd/datadel/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --dry-run \
  --verbose

# 真實刪除資料，使用批次模式
go run ./cmd/datadel/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --batch
```

---

## 完整測試流程範例

以下是完整的測試流程示範：

1. 啟動 DynamoDB Migration Monitor 進行監控：

```bash
go run . \
  --source-profile 5566_dev \
  --target-profile 5566_dev \
  --stream-profile 5566_dev \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:5566:table/test_ddb/stream/2025-05-20T13:05:28.798" \
  --target-table "test_ddb" \
  --partition-key "pk" \
  --sort-key "sk" \
  --region ap-northeast-1 \
  --sample-rate 1 \
  --verify-on source \
  --iterator-type TRIM_HORIZON \
  --verbose true
```

2. 在另一個終端機視窗產生測試資料：

```bash
go run ./cmd/datagen/main.go \
  --profile 5566_dev \
  --table test_ddb \
  --count 100 \
  --wait 100 \
  --output ./test_keys.csv
```

3. 觀察 Monitor 是否能夠正確處理串流事件

4. 測試完成後清理測試資料：

```bash
go run ./cmd/datadel/main.go \
  --profile 5566_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --batch
``` 