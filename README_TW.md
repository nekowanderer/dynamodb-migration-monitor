# DynamoDB Migration Monitor

[English](README.md) | [繁體中文](README_TW.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

此專案用於監控和驗證 DynamoDB 表格遷移的過程。透過監聽目標表格（Target Table）的 DynamoDB Stream，我們可以即時追蹤資料遷移的狀態並進行抽樣驗證。

有關監控機制的技術詳細資訊，請參閱[監控機制](monitoring_mechanism_tw.md)。

## 監控方式

目前支援：
- 基於 Stream 的監控：使用 DynamoDB Streams 即時追蹤資料變更

未來計畫：
- 基於表格查詢的監控：將支援直接比對表格資料的方式

## 遷移架構

遷移架構採用 S3 匯出/匯入結合 DynamoDB Stream 的方式，以實現最小停機時間的資料遷移。

### 遷移時間軸

```
T0 ─────────── T1 ─────────── T2 ─────────── T3 ─────────── T4
   ^            ^             ^              ^              ^
   |            |             |              |              |
   |            |             |              |              |
啟用 Stream    開始 S3 匯出   S3 匯入完成     複寫完成        驗證完成

時間點說明：
T0：啟用 DynamoDB Stream，開始追蹤所有資料變更
T1：開始將資料匯出至 S3，這是基準資料快照的時間點
T2：S3 資料匯入完成，但可能有 T1-T2 期間的資料變更尚未同步
T3：複寫程序完成，所有 T1-T2 期間的變更都已同步
T4：資料驗證完成，確認所有資料都已正確遷移
```

### 遷移步驟

1. **啟用 DynamoDB Stream (T0)**
   - 在來源表格啟用 DynamoDB Stream
   - Stream 類型設定為 `NEW_IMAGES`
   - 此時開始記錄所有資料變更

2. **匯出資料至 S3 (T1)**
   - 從來源帳號將表格資料匯出到 S3
   - 這是資料的基準快照

3. **從 S3 匯入資料 (T1 ~ T2)**
   - 將 S3 的資料匯入到目標帳號的表格
   - 這個過程可能需要較長時間，取決於資料量

4. **資料複寫 (T1 ~ T3)**
   - DBA 的複寫腳本會處理 T1 到 T2 期間累積的變更
   - 確保在 S3 匯出/匯入期間的資料變更不會遺失
   - 等待複寫腳本消化完所有累積的變更

5. **設定即時複寫 (T3)**
   - 確認即時複寫機制正常運作
   - 此時新的變更應該能即時同步

6. **資料驗證 (T0 ~ T4)**
   - 使用本工具進行資料驗證
   - 驗證策略：監聽來源表格變更，然後查詢目標表格
   - 這種方式比監聽目標表格更準確

```
來源表格 ──── S3 匯出 ────┐
   │                     │
   │                     v
   │               目標表格 (S3 匯入)
   │                     │
   └──── Stream ─────────┘
         (即時複寫)
```

### 驗證策略

本工具採用以下驗證方式：

1. **主要策略（目前採用）**
   - 監聽來源表格的 Stream
   - 對每個變更，查詢目標表格驗證資料
   - 優點：
     - 可以準確追蹤所有來源資料的變更
     - 不會遺漏任何資料變更
     - 可以即時發現同步延遲或失敗
     - 適合長期運行的遷移專案

2. **替代策略**
   - 監聽目標表格的 Stream
   - 對每個變更，查詢來源表格驗證資料
   - 缺點：
     - 可能無法完整追蹤所有資料變更
     - 無法確保來源表格的所有變更都有被複製
     - 較難發現同步失敗的情況
     - 不適合需要高準確度的遷移專案

### 為什麼選擇這個架構？

1. **最小化停機時間**
   - 使用 S3 進行大量資料遷移
   - Stream 處理遷移期間的增量更新
   - 無需停止應用程式寫入

2. **資料一致性保證**
   - Stream 確保捕獲所有資料變更
   - 複寫腳本處理累積的變更
   - 驗證工具確保資料正確性

3. **可靠的驗證機制**
   - 即時監控資料變更
   - 支援抽樣驗證降低成本
   - 詳細的驗證統計和日誌

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
  --stream-profile <stream_profile> \
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
  --stream-profile stream_profile \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:123456789012:table/my-table/stream/2024-01-01T00:00:00.000" \
  --target-table "my-table" \
  --partition-key "user_id" \
  --sort-key "timestamp" \
  --region ap-northeast-1 \
  --sample-rate 50 \
  --verify-on source \
  --verbose
```

### 參數說明

| 參數 | 必填 | 預設值 | 說明 | 可能的值 |
|------|------|--------|------|----------|
| `--source-profile` | 是 | - | 來源 AWS profile 名稱，用於存取來源表格 | 任何已設定的 AWS profile |
| `--target-profile` | 是 | - | 目標 AWS profile 名稱，用於存取目標表格 | 任何已設定的 AWS profile |
| `--stream-arn` | 是 | - | 目標表格的 Stream ARN，用於監控資料變更 | 例如："arn:aws:dynamodb:region:account:table/name/stream/time" |
| `--target-table` | 是 | - | 目標表格名稱，即要驗證的目標表格 | 任何 DynamoDB 表格名稱 |
| `--partition-key` | 是 | - | 分區鍵名稱，用於資料查詢和比對 | 任何有效的分區鍵名稱 |
| `--stream-profile` | 否 | 同 source-profile | Stream AWS profile 名稱。如果 Stream 存取需要不同的權限設定，可以指定專用的 profile | 任何已設定的 AWS profile |
| `--sort-key` | 否 | - | 排序鍵名稱（如果表格有的話）。用於複合主鍵的情況 | 任何有效的排序鍵名稱 |
| `--region` | 否 | ap-northeast-1 | AWS Region。指定要操作的 AWS 區域 | 任何有效的 AWS 區域 |
| `--sample-rate` | 否 | 100 | 驗證抽樣率。可以降低以減少成本 | 任何正整數 |
| `--verify-on` | 否 | source | 指定要驗證的表格：source 或 target | "source", "target" |
| `--verbose` | 否 | false | 顯示成功驗證的日誌。開啟後可以看到所有驗證細節，但可能會有大量輸出 | true, false |

## 關於抽樣率和統計可信度

即使僅 **1% 抽樣率**，在此規模下依然具備極高的統計意義。以 **3,000 萬筆** 資料為例：

| 面向 | 1% 抽樣（30 萬筆樣本） |
|------|-------------------------|
| 樣本絕對數量 | 30 萬屬於「極大樣本」，遠高於多數學術研究所需。 |
| 誤差範圍（95% 信賴區間） | 約 ±0.18%。若樣本顯示 **99%** 成功，真實比例幾乎可確定介於 **98.82%** 與 **99.18%** 之間。 |
| 問題偵測能力 | 即使問題率僅 **0.1%**，30 萬樣本中仍會出現 ≈ 300 筆錯誤，足以被偵測並分析。 |
| 實務考量 | 遷移問題通常屬於 **系統性**，非隨機事件；若 1% 抽樣皆正常，出現隱藏系統性問題的機率極低。 |

綜合而言，1% 抽樣在成本效益與準確性之間取得了極佳平衡；若驗證過程發現異常，可隨時提高抽樣率以進一步深入分析。

### Stream Style Verification

基於 Stream 的驗證模式會在資料遷移過程中即時監控 DynamoDB Streams。它包含了幾個重要的功能來處理最終一致性和資料複寫延遲：

#### 驗證流程

1. **批次處理**
   - 記錄會被收集在緩衝區中（預設大小：100）
   - 以批次方式處理以減少 API 呼叫
   - 可透過環境變數設定批次大小

2. **複寫延遲處理**
   - 等待資料複寫完成（預設：5 秒）
   - 確保資料在目標表格中可用
   - 協助處理 DynamoDB 的最終一致性

3. **重試機制**
   - 對失敗的驗證實作自動重試
   - 重試之間等待（預設：2 秒）
   - 協助處理暫時性的複寫延遲

4. **非同步處理**
   - 使用通道進行並行驗證
   - 防止阻塞主要的串流處理
   - 可配置的通道大小（預設：10）

#### 配置參數

以下環境變數可用於調整驗證流程：

| 環境變數 | 預設值 | 說明 |
|----------|--------|------|
| `DDB_VALIDATION_BUFFER_SIZE` | 100 | 驗證緩衝區大小 |
| `DDB_VALIDATION_CHANNEL_SIZE` | 10 | 驗證通道大小 |
| `DDB_VALIDATION_INTERVAL` | 30s | 處理驗證緩衝區的間隔 |
| `DDB_REPLICATION_WAIT_TIME` | 5s | 等待資料複寫的時間 |
| `DDB_RETRY_WAIT_TIME` | 2s | 重試前的等待時間 |
| `DDB_BATCH_SIZE` | 100 | 串流批次大小 |
| `DDB_STATS_INTERVAL` | 30s | 顯示統計資訊的間隔 |

#### 為什麼需要這些功能？

在進行大規模遷移（數千萬筆資料）時，我們觀察到：

1. **最終一致性**
   - DynamoDB 的最終一致性模型意味著資料複寫有固有的延遲
   - 來源表格的變更可能不會立即在目標表格中可見
   - 在高吞吐量的遷移中特別明顯

2. **複寫延遲**
   - 來源表格的變更需要時間才會出現在目標表格中
   - 簡單的立即驗證會產生誤判
   - 批次處理配合等待時間可以處理這些延遲

3. **效能考量**
   - 批次處理減少 API 呼叫
   - 非同步驗證防止阻塞
   - 可配置的參數允許針對不同場景進行調整

## 監控輸出

程式會每 30 秒顯示一次統計資訊，包含：
- 總事件數和唯一事件數
- INSERT 和 MODIFY 操作的數量
- 每秒平均事件數
- 驗證成功率

## 架構與 IAM 設定

### 跨帳號存取架構

本工具可以在 EC2 執行個體上運行，使用專用的 IAM 角色來存取來源和目標帳號的資源。以下是架構圖：

```
+------------------------------------------------------+
|                                                      |
|  EC2 (with temp-ddb-migration-role)                  |
|  +------------------------------------------+        |
|  |                                          |        |
|  |                  驗證腳本                 |        |
|  |                                          |        |
|  +------------------------------------------+        |
|          |                     |                     |
|          | 讀取                 | 驗證                |
|          | Stream              | 目標                 |
|          ↓                     ↓                     |
+------------------------------------------------------+
          |                     |
          |                     |
          |                     |
          |                     |
          ↓                     ↓
+-------------------+  +-------------------+
|                   |  |                   |
|      來源帳號      |  |     目標帳號        |
| (paul-leishman-qa)|  |   (codashop-qa)   |
| 169579254xxx      |  |   042913693xxx    |
|                   |  |                   |
| +---------------+ |  | +---------------+ |
| |               | |  | |               | |
| | DynamoDB      | |  | | DynamoDB      | |
| | Stream        | |  | | 表格           | |
| |               | |  | |               | |
| +---------------+ |  | +---------------+ |
|                   |  |                   |
+-------------------+  +-------------------+
```

### 資料流程

1. 腳本從來源帳號的 DynamoDB Stream 讀取記錄
2. 對每個串流記錄，腳本查詢目標帳號的表格
3. 驗證記錄是否正確遷移到目標表格
4. EC2 角色需要權限來存取兩個帳號的資源

### 必要的 IAM 權限

#### 1. EC2 執行個體角色 (temp-ddb-migration-role)

此角色需要權限來讀取來源串流和目標表格：

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:Scan"
      ],
      "Resource": "arn:aws:dynamodb:ap-southeast-1:042913693xxx:table/staging-codashop-userdetails"
    },
    {
      "Effect": "Allow",
      "Action": [
        "dynamodbstreams:DescribeStream",
        "dynamodbstreams:GetRecords",
        "dynamodbstreams:GetShardIterator",
        "dynamodbstreams:ListStreams"
      ],
      "Resource": "arn:aws:dynamodb:ap-southeast-1:169579254xxx:table/staging-codashop-userdetails/stream/*"
    }
  ]
}
```

#### 2. 來源帳號 (paul-leishman-qa, 169579254xxx)

需要允許遷移角色存取串流：

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::042913693xxx:role/temp-ddb-migration-role"
      },
      "Action": [
        "dynamodbstreams:DescribeStream",
        "dynamodbstreams:GetRecords",
        "dynamodbstreams:GetShardIterator",
        "dynamodbstreams:ListStreams"
      ],
      "Resource": "arn:aws:dynamodb:ap-southeast-1:169579254xxx:table/staging-codashop-userdetails/stream/*"
    }
  ]
}
```

#### 3. 目標帳號 (codashop-qa, 042913693xxx)

需要允許遷移角色存取表格：

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::042913693xxx:role/temp-ddb-migration-role"
      },
      "Action": [
        "dynamodb:GetItem",
        "dynamodb:Query",
        "dynamodb:Scan"
      ],
      "Resource": "arn:aws:dynamodb:ap-southeast-1:042913693xxx:table/staging-codashop-userdetails"
    }
  ]
}
```

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
