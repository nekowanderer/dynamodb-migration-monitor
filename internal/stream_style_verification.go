package internal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	streamtypes "github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	log "github.com/sirupsen/logrus"
)

// StreamVerificationConfig contains all the configuration needed for stream verification
type StreamVerificationConfig struct {
	SourceClient *dynamodb.Client
	TargetClient *dynamodb.Client
	StreamClient *dynamodbstreams.Client
	StreamArn    string
	TargetTable  string
	SampleRate   int    // Validate 1 out of every SampleRate records
	PartitionKey string // Name of the partition key
	SortKey      string // Name of the sort key (optional)
	IteratorType string // DynamoDB Stream Iterator Type
	VerifyOn     string // Which table to verify against: source or target
	Verbose      bool   // Whether to show success validation logs
}

// Stats tracks stream processing statistics
type Stats struct {
	InsertCount       int
	ModifyCount       int
	TotalCount        int
	StartTime         time.Time
	EventIDs          map[string]struct{} // For deduplication
	ValidationCount   int                 // Number of records validated
	ValidationSuccess int                 // Records successfully validated
	ValidationFailed  int                 // Records that failed validation
}

// RunStreamStyleVerification sets up and runs the stream-based verification process
func RunStreamStyleVerification(ctx context.Context, cfg *StreamVerificationConfig) {
	// Set default sample rate if not provided
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 100 // Default: validate 1 out of every 100 records
	}

	// Select client based on VerifyOn setting
	verifiedClient := cfg.TargetClient // Default to target client
	verifiedTable := cfg.TargetTable
	if cfg.VerifyOn == "source" {
		verifiedClient = cfg.SourceClient
		// We still use the target table name since it's the same structure in both accounts
	}

	// Using StreamSubscriberV2 to directly listen to DynamoDB Stream
	subscriber := NewStreamSubscriberV2(verifiedClient, cfg.StreamClient, verifiedTable)

	// Set iterator type based on configuration
	if cfg.IteratorType == "TRIM_HORIZON" {
		subscriber.SetShardIteratorType(streamtypes.ShardIteratorTypeTrimHorizon)
	} else {
		subscriber.SetShardIteratorType(streamtypes.ShardIteratorTypeLatest)
	}

	// To speed up reading, you can set the batch size
	subscriber.SetLimit(100)

	recCh, errCh := subscriber.GetStreamDataAsync()

	// Listen for OS interrupt to gracefully shut down on Ctrl+C
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Counters and statistics
	stats := &Stats{
		StartTime: time.Now(),
		EventIDs:  make(map[string]struct{}),
	}

	// Timer to display statistics every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Function to verify data in table
	verifyInTable := func(ctx context.Context, partitionKeyValue, sortKeyValue string) bool {
		// Create GetItem input for table
		keys := map[string]types.AttributeValue{
			cfg.PartitionKey: &types.AttributeValueMemberS{Value: partitionKeyValue},
		}

		// Add sort key if configured
		if cfg.SortKey != "" && sortKeyValue != "" {
			keys[cfg.SortKey] = &types.AttributeValueMemberS{Value: sortKeyValue}
		}

		// Select client and table based on VerifyOn setting
		client := cfg.SourceClient
		tableType := "source"
		tableName := cfg.TargetTable // We use target table name for both directions as it should have same structure

		if cfg.VerifyOn == "target" {
			client = cfg.TargetClient
			tableType = "target"
		}

		input := &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key:       keys,
		}

		// Query the table
		result, err := client.GetItem(ctx, input)
		if err != nil {
			log.WithFields(log.Fields{
				"partition_key": fmt.Sprintf("%s=%s", cfg.PartitionKey, partitionKeyValue),
				"sort_key":      fmt.Sprintf("%s=%s", cfg.SortKey, sortKeyValue),
				"error":         err,
			}).Warn("[VALIDATION] Error querying " + tableType + " table")
			return false
		}

		// Check if item exists in table
		exists := len(result.Item) > 0

		if exists {
			if cfg.Verbose {
				log.WithFields(log.Fields{
					"partition_key": fmt.Sprintf("%s=%s", cfg.PartitionKey, partitionKeyValue),
					"sort_key":      fmt.Sprintf("%s=%s", cfg.SortKey, sortKeyValue),
				}).Info("[VALIDATION] SUCCESS: Item exists in " + tableType + " table ✅")
			}
		} else {
			log.WithFields(log.Fields{
				"partition_key": fmt.Sprintf("%s=%s", cfg.PartitionKey, partitionKeyValue),
				"sort_key":      fmt.Sprintf("%s=%s", cfg.SortKey, sortKeyValue),
			}).Warn("[VALIDATION] FAILED: Item not found in " + tableType + " table ❌")
		}

		return exists
	}

	// Print statistics
	printStats := func() {
		duration := time.Since(stats.StartTime)
		log.Infof("========= Stream Event Statistics (Total %s) =========", duration.Round(time.Second))
		log.Infof("Total events: %d (Unique: %d)", stats.TotalCount, len(stats.EventIDs))
		log.Infof("INSERT: %d, MODIFY: %d", stats.InsertCount, stats.ModifyCount)
		log.Infof("Average: %.2f events/sec", float64(stats.TotalCount)/duration.Seconds())

		// Add validation statistics
		if stats.ValidationCount > 0 {
			successRate := float64(stats.ValidationSuccess) / float64(stats.ValidationCount) * 100
			log.Infof("Validation: %d sampled, %d success (%.1f%%), %d failed",
				stats.ValidationCount, stats.ValidationSuccess, successRate, stats.ValidationFailed)
		}

		log.Infof("========================================")
	}

	for {
		select {
		case rec := <-recCh:
			// Ignore Remove type events
			if rec.EventName == streamtypes.OperationTypeRemove {
				continue
			}

			stats.TotalCount++
			eventID := aws.ToString(rec.EventID)

			// Count event types
			switch rec.EventName {
			case streamtypes.OperationTypeInsert:
				stats.InsertCount++
			case streamtypes.OperationTypeModify:
				stats.ModifyCount++
			}

			// Record unique events
			stats.EventIDs[eventID] = struct{}{}

			// Extract keys from the record
			var partitionKeyValue, sortKeyValue string

			if rec.Dynamodb != nil && rec.Dynamodb.Keys != nil {
				// Extract Partition Key
				if pkAttr, ok := rec.Dynamodb.Keys[cfg.PartitionKey]; ok {
					if pkStr, ok := pkAttr.(*streamtypes.AttributeValueMemberS); ok {
						partitionKeyValue = pkStr.Value
					}
				}

				// Extract Sort Key if configured
				if cfg.SortKey != "" {
					if skAttr, ok := rec.Dynamodb.Keys[cfg.SortKey]; ok {
						if skStr, ok := skAttr.(*streamtypes.AttributeValueMemberS); ok {
							sortKeyValue = skStr.Value
						}
					}
				}
			}

			log.WithFields(log.Fields{
				"event_id":      eventID,
				"event_type":    rec.EventName,
				"partition_key": fmt.Sprintf("%s=%s", cfg.PartitionKey, partitionKeyValue),
				"sort_key":      fmt.Sprintf("%s=%s", cfg.SortKey, sortKeyValue),
			}).Info("[STREAM] Record received")

			// Validate if sampling conditions are met
			if stats.TotalCount%cfg.SampleRate == 0 && partitionKeyValue != "" {
				// Perform validation against the table
				stats.ValidationCount++

				// Validate in table
				success := verifyInTable(ctx, partitionKeyValue, sortKeyValue)
				if success {
					stats.ValidationSuccess++
				} else {
					stats.ValidationFailed++
				}
			}
		case err := <-errCh:
			log.Errorf("[STREAM] Error: %v", err)
		case <-ticker.C:
			printStats()
		case <-c:
			log.Info("Interrupt received, shutting down stream listener...")
			printStats() // Show final statistics before exiting
			return
		case <-ctx.Done():
			log.Info("Context canceled, shutting down stream listener...")
			printStats() // Show final statistics before exiting
			return
		}
	}
}
