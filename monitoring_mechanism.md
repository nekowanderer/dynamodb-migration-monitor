# Monitoring Mechanism

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

[English](monitoring_mechanism.md) | [繁體中文](monitoring_mechanism_tw.md)

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

`internal/stream_style_verification.go` consumes records via `GetStreamDataAsync()`, selecting the appropriate client based on the `verifyOn` parameter:
```go
// Select client based on VerifyOn setting
verifiedClient := cfg.TargetClient  // Default to target client
verifiedTable := cfg.TargetTable
if cfg.VerifyOn == "source" {
    verifiedClient = cfg.SourceClient
    // We still use the target table name since it's the same structure in both accounts
}

subscriber := NewStreamSubscriberV2(verifiedClient, cfg.StreamClient, verifiedTable)
subscriber.SetLimit(100)
recCh, errCh := subscriber.GetStreamDataAsync()
```

Records then flow into the main `select` loop for deduplication and sampling validation.

## 4. Future Enhancements

* **Persisted Offsets**: Store iterator progress in DynamoDB/Redis for resume after restart.
* **Enhanced Error Handling**: Sophisticated retry/back-off for errors other than `TrimmedDataAccessException`.
* **Table Query Mode**: In addition to streams, direct Query/Scan comparison between source & target tables.

---

> References
> * AWS Developer Guide – [Working with Shards](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/Streams.html)
> * AWS re:Invent – *Advanced Patterns for DynamoDB Streams* 