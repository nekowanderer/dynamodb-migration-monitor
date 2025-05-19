package internal

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	stypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/aws/smithy-go"
)

// Usage:
//  sub := NewStreamSubscriberV2(ddbClient, streamClient, "table_name")
//  recCh, errCh := sub.GetStreamData()  // or GetStreamDataAsync()
//  for r := range recCh { ... }
//  // Receive from errCh to avoid goroutine leaks
//
//  Currently uses context.Background() internally. If more control is needed,
//  consider modifying the implementation to accept a context parameter.
//
//  Note: This implementation only covers the most common use cases and doesn't
//  handle all AWS error types. If you need to support errors other than
//  TrimmedDataAccess, please extend the implementation.

type StreamSubscriberV2 struct {
	dynamoSvc *dynamodb.Client
	streamSvc *dynamodbstreams.Client
	table     string

	ShardIteratorType stypes.ShardIteratorType
	Limit             *int32
}

func NewStreamSubscriberV2(
	dynamoSvc *dynamodb.Client,
	streamSvc *dynamodbstreams.Client,
	table string,
) *StreamSubscriberV2 {
	s := &StreamSubscriberV2{
		dynamoSvc: dynamoSvc,
		streamSvc: streamSvc,
		table:     table,
	}
	s.applyDefaults()
	return s
}

func (s *StreamSubscriberV2) applyDefaults() {
	if s.ShardIteratorType == "" {
		s.ShardIteratorType = stypes.ShardIteratorTypeLatest
	}
}

func (s *StreamSubscriberV2) SetLimit(v int32) {
	s.Limit = aws.Int32(v)
}

func (s *StreamSubscriberV2) SetShardIteratorType(t stypes.ShardIteratorType) {
	s.ShardIteratorType = t
}

// GetStreamData follows the same logic as the original implementation:
// 1. Find the "latest" or "next" Shard.
// 2. Read data sequentially and send it to the Channel.
// 3. If the Shard is closed (Iterator == nil), sleep for 10ms and retry.
func (s *StreamSubscriberV2) GetStreamData() (<-chan *stypes.Record, <-chan error) {
	recCh := make(chan *stypes.Record, 1)
	errCh := make(chan error, 1)

	go func() {
		var shardID *string
		var prevShardID *string
		var arn *string
		var err error

		ctx := context.Background()

		for {
			prevShardID = shardID
			shardID, arn, err = s.findProperShardID(ctx, prevShardID)
			if err != nil {
				errCh <- err
			}
			if shardID != nil {
				if err = s.processShard(ctx, &dynamodbstreams.GetShardIteratorInput{
					StreamArn:         arn,
					ShardId:           shardID,
					ShardIteratorType: s.ShardIteratorType,
				}, recCh); err != nil {
					errCh <- err
					// Process the same shard again
					shardID = prevShardID
				}
			}
			if shardID == nil {
				time.Sleep(10 * time.Second)
			}
		}
	}()

	return recCh, errCh
}

// GetStreamDataAsync can process multiple Shards concurrently and checks for new Shards
// periodically (every 1m). Default concurrency limit is 5.
func (s *StreamSubscriberV2) GetStreamDataAsync() (<-chan *stypes.Record, <-chan error) {
	recCh := make(chan *stypes.Record, 1)
	errCh := make(chan error, 1)

	needUpdate := make(chan struct{}, 1)
	needUpdate <- struct{}{}

	allShards := make(map[string]struct{})
	shardProcessingLimit := 5
	shardsCh := make(chan *dynamodbstreams.GetShardIteratorInput, shardProcessingLimit)
	var lock sync.Mutex

	// Push update request once per minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			needUpdate <- struct{}{}
		}
	}()

	// Listen for update signals and generate shards to process
	go func() {
		ctx := context.Background()
		for range needUpdate {
			arn, err := s.getLatestStreamArn(ctx)
			if err != nil {
				errCh <- err
				return
			}
			ids, err := s.getShardIDs(ctx, arn)
			if err != nil {
				errCh <- err
				return
			}
			for _, shard := range ids {
				lock.Lock()
				if _, ok := allShards[*shard.ShardId]; !ok {
					allShards[*shard.ShardId] = struct{}{}
					shardsCh <- &dynamodbstreams.GetShardIteratorInput{
						StreamArn:         arn,
						ShardId:           shard.ShardId,
						ShardIteratorType: s.ShardIteratorType,
					}
				}
				lock.Unlock()
			}
		}
	}()

	limit := make(chan struct{}, shardProcessingLimit)

	go func() {
		time.Sleep(10 * time.Second)
		for shardInput := range shardsCh {
			limit <- struct{}{}
			go func(input *dynamodbstreams.GetShardIteratorInput) {
				ctx := context.Background()
				if err := s.processShard(ctx, input, recCh); err != nil {
					errCh <- err
				}
				<-limit
			}(shardInput)
		}
	}()

	return recCh, errCh
}

// ----------------- Private Helper Methods -----------------

func (s *StreamSubscriberV2) getShardIDs(ctx context.Context, streamArn *string) ([]stypes.Shard, error) {
	out, err := s.streamSvc.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	})
	if err != nil {
		return nil, err
	}
	if out.StreamDescription == nil || len(out.StreamDescription.Shards) == 0 {
		return nil, nil
	}
	return out.StreamDescription.Shards, nil
}

func (s *StreamSubscriberV2) findProperShardID(ctx context.Context, prevShardID *string) (*string, *string, error) {
	arn, err := s.getLatestStreamArn(ctx)
	if err != nil {
		return nil, nil, err
	}
	out, err := s.streamSvc.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{StreamArn: arn})
	if err != nil {
		return nil, nil, err
	}
	if out.StreamDescription == nil || len(out.StreamDescription.Shards) == 0 {
		return nil, nil, nil
	}

	// If there's no previous shard, return the latest one
	if prevShardID == nil {
		return out.StreamDescription.Shards[0].ShardId, arn, nil
	}

	for _, shard := range out.StreamDescription.Shards {
		if shard.ParentShardId != nil && *shard.ParentShardId == *prevShardID {
			return shard.ShardId, arn, nil
		}
	}
	return nil, nil, nil
}

func (s *StreamSubscriberV2) getLatestStreamArn(ctx context.Context) (*string, error) {
	out, err := s.dynamoSvc.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(s.table)})
	if err != nil {
		return nil, err
	}
	if out.Table == nil || out.Table.LatestStreamArn == nil {
		return nil, errors.New("empty table stream arn")
	}
	return out.Table.LatestStreamArn, nil
}

func (s *StreamSubscriberV2) processShard(ctx context.Context, input *dynamodbstreams.GetShardIteratorInput, recCh chan<- *stypes.Record) error {
	iterOut, err := s.streamSvc.GetShardIterator(ctx, input)
	if err != nil {
		return err
	}
	if iterOut.ShardIterator == nil {
		return nil
	}

	next := iterOut.ShardIterator

	for next != nil {
		recOut, err := s.streamSvc.GetRecords(ctx, &dynamodbstreams.GetRecordsInput{
			ShardIterator: next,
			Limit:         s.Limit,
		})
		if err != nil {
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "TrimmedDataAccessException" {
				// Trying to read data older than 24h, can be safely ignored
				return nil
			}
			return err
		}

		for i := range recOut.Records {
			// Address the record to avoid concurrency issues
			rec := recOut.Records[i]
			recCh <- &rec
		}

		next = recOut.NextShardIterator

		sleep := time.Second
		if next == nil {
			sleep = 10 * time.Millisecond
		} else if len(recOut.Records) == 0 {
			sleep = 10 * time.Second
		}
		time.Sleep(sleep)
	}
	return nil
}
