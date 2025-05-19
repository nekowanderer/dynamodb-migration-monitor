# Monitoring Mechanism 監控機制

[English](#english-version) | [中文](#中文版本)

---

# English Version

## 1. DynamoDB Stream & Shard Basics

AWS DynamoDB Streams write every data mutation (INSERT / UPDATE / DELETE) into **shards**—similar to Kinesis partitions.

* Each shard holds an ordered sequence of events.
* A shard comes with a `SequenceNumberRange` indicating its event range.
* When traffic grows, DynamoDB may **split** a shard or **merge** shards, creating a **parent / child** relationship.
  * The original shard becomes **closed** (no new events).
  * New shards—called **leaf shards**—start receiving events.

Official references:
* [DynamoDB Streams Developer Guide – Shards](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html)

Because shards can appear or close at any time, a monitor must:
1. Periodically fetch the latest Stream ARN & shard list.
2. Start a consumer for every unseen leaf shard.
3. Prevent duplicate reads and gracefully exit when a shard is closed.

## 2. Implementation in This Project

### 2.1 Shard Discovery & Refresh

`internal/stream_subscriber_v2.go` drives the logic in two stages:

1. **Periodic refresh (every minute)**
   ```go
   // Push update request once per minute
   go func() {
       ticker := time.NewTicker(time.Minute)
       for range ticker.C {
           needUpdate <- struct{}{}
       }
   }()
   ```
2. **Diff current shards with the newest list** and start a reader only for new leaf shards:
   ```go
   ids, err := s.getShardIDs(ctx, arn)
   for _, shard := range ids {
       if _, ok := allShards[*shard.ShardId]; !ok {
           allShards[*shard.ShardId] = struct{}{}
           shardsCh <- &dynamodbstreams.GetShardIteratorInput{ ... }
       }
   }
   ```

### 2.2 Concurrent Shard Processing

* `shardProcessingLimit` (default **5**) caps concurrent shard readers.
* Each shard is read via long-polling `GetRecords` in `processShard`:
  ```go
  recOut, err := s.streamSvc.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{ ... })
  for i := range recOut.Records {
      rec := recOut.Records[i]
      recCh <- &rec
  }
  ```
* When `NextShardIterator` is `nil`, the shard is closed—goroutine exits automatically.

### 2.3 Parent / Child Shard Handoff

`findProperShardID` decides which shard to read next:
* **Startup** → pick the latest leaf shard.
* When current shard closes → find its child shard (`ParentShardId == prevShardID`).

```go
for _, shard := range out.StreamDescription.Shards {
    if shard.ParentShardId != nil && *shard.ParentShardId == *prevShardID {
        return shard.ShardId, arn, nil
    }
}
```

This guarantees the consumer migrates to new shards after a split.

## 3. Integration with Main Verification Logic

`internal/stream_style_verification.go` consumes records via `GetStreamDataAsync()`:
```go
subscriber := NewStreamSubscriberV2(cfg.TargetClient, cfg.StreamClient, cfg.TargetTable)
subscriber.SetLimit(100)
recCh, errCh := subscriber.GetStreamDataAsync()
```

Records then flow into the main `select` loop for deduplication and sampling validation.

## 4. Future Enhancements

* **Persisted Offsets**: Store iterator progress in DynamoDB/Redis for resume after restart.
* **Enhanced Error Handling**: Sophisticated retry/back-off for errors other than `TrimmedDataAccessException`.
* **Table Query Mode**: In addition to streams, direct Query/Scan comparison between source & target tables.

---

# 中文版本

## 1. DynamoDB Stream 與 Shard 基礎

AWS DynamoDB Stream 會將每一筆資料變更 (INSERT/UPDATE/DELETE) 以事件形式寫入 **Shard**。

* 一個 Shard 相當於 Kinesis 的 Partition，內含多條有序的事件記錄。
* 每個 Shard 具有 `SequenceNumberRange`，代表該 Shard 事件的起迄點。
* Shard 會因為 **水平擴充 (split)** 或 **合併 (merge)** 而產生「父/子關係」。
  * 舊的 Shard 會變成 **Closed**，不再寫入新事件。
  * 新產生的 Shard 稱為 **Leaf Shard**，開始接收事件。

官方文件：
* [DynamoDB Streams Developer Guide – Shard](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html)

由於 Shard 可能隨著流量自動切割或合併，**監控程式必須：**
1. 定期取得最新的 Stream ARN 與 Shard 列表。
2. 確保每個尚未處理過的 Leaf Shard 都被啟動監聽。
3. 處理過的 Shard 需避免重複讀取，並在 Shard 終結後安全關閉。

## 2. 本專案的實作策略

### 2.1 Shard 追蹤與更新

程式以 `internal/stream_subscriber_v2.go` 為核心，採用兩階段機制：

1. **定時 (每分鐘) 更新 Shard**
   ```go
   // Push update request once per minute
   go func() {
       ticker := time.NewTicker(time.Minute)
       for range ticker.C {
           needUpdate <- struct{}{}
       }
   }()
   ```
2. **比對已知 Shard 與最新清單**，只針對新的 Leaf Shard 建立 Iterator：
   ```go
   ids, err := s.getShardIDs(ctx, arn)
   for _, shard := range ids {
       if _, ok := allShards[*shard.ShardId]; !ok {
           allShards[*shard.ShardId] = struct{}{}
           shardsCh <- &dynamodbstreams.GetShardIteratorInput{ ... }
       }
   }
   ```

### 2.2 併發處理 Shard

* 透過 `shardProcessingLimit` (預設 5) 控制同時處理的 Shard 數量，避免過度佔用連線與 API 配額。
* 每個 Shard 由 `processShard` 以 **長輪詢** (GetRecords) 方式讀取：
  ```go
  recOut, err := s.streamSvc.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{ ... })
  for i := range recOut.Records {
      rec := recOut.Records[i]
      recCh <- &rec
  }
  ```
* 如果 `NextShardIterator` 為 `nil` 代表 Shard 已關閉；程式將自動離開迴圈並釋放 goroutine。

### 2.3 處理父 / 子 Shard

函式 `findProperShardID` 會判斷：
* **第一次啟動** 取得最新 Leaf Shard。
* 若目前 Shard 終結，便尋找其子 Shard (ParentShardId == prevShardID)。

```go
for _, shard := range out.StreamDescription.Shards {
    if shard.ParentShardId != nil && *shard.ParentShardId == *prevShardID {
        return shard.ShardId, arn, nil
    }
}
```

如此可確保在 Shard split 後，監聽自動轉移至子 Shard。

## 3. 與主程式的整合

在 `internal/stream_style_verification.go` 中，透過 `GetStreamDataAsync()` 取得事件：
```go
subscriber := NewStreamSubscriberV2(cfg.TargetClient, cfg.StreamClient, cfg.TargetTable)
subscriber.SetLimit(100)
recCh, errCh := subscriber.GetStreamDataAsync()
```

事件會進入主 `select` 迴圈，進一步做去重與抽樣驗證。

## 4. 未來改進方向

* **Persist Offset**：目前 Iterator 進度存在記憶體，預期未來可寫入 DynamoDB/Redis 供重啟續讀。
* **Enhanced Error Handling**：對 `TrimmedDataAccessException` 以外的錯誤提供更細緻重試策略。
* **Table Query Mode**：除了 Stream，也將支援直接 Query/Scan 方式對比 Source/Target 表。

---

> References
> * AWS Developer Guide – [Working with Shards](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html)
> * AWS re:Invent – *Advanced Patterns for DynamoDB Streams* 