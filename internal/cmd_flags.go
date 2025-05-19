package internal

import (
	"errors"
	"flag"
)

// CommandFlags contains all command line parameters
type CommandFlags struct {
	SourceProfile string // Source AWS profile name
	TargetProfile string // Target AWS profile name
	StreamArn     string // DynamoDB Stream ARN (optional)
	TargetTable   string // Target DynamoDB table name
	PartitionKey  string // Name of the partition key
	SortKey       string // Name of the sort key (optional)
	Region        string // AWS Region (optional, defaults to ap-northeast-1)
	SampleRate    int    // Validate 1 out of every SampleRate records (optional, defaults to 100)
}

// ParseCommandFlags parses command line flags and returns the configuration
func ParseCommandFlags() (*CommandFlags, error) {
	sourceProfilePtr := flag.String("source-profile", "", "Source AWS profile name (required)")
	targetProfilePtr := flag.String("target-profile", "", "Target AWS profile name (required)")
	streamArnPtr := flag.String("stream-arn", "", "DynamoDB Stream ARN for stream-based count check (optional)")
	targetTablePtr := flag.String("target-table", "", "Target DynamoDB table name for stream-based check (required if stream-arn is set)")
	partitionKeyPtr := flag.String("partition-key", "", "Name of the partition key (required if stream-arn is set)")
	sortKeyPtr := flag.String("sort-key", "", "Name of the sort key (optional)")
	regionPtr := flag.String("region", "ap-northeast-1", "AWS Region (optional, defaults to ap-northeast-1)")
	sampleRatePtr := flag.Int("sample-rate", 100, "Validate 1 out of every N records (optional, defaults to 100)")
	flag.Parse()

	// Validate required flags
	if *sourceProfilePtr == "" || *targetProfilePtr == "" {
		return nil, errors.New("missing required flags: source-profile and target-profile are required")
	}

	// Additional validation
	if *streamArnPtr != "" {
		if *targetTablePtr == "" {
			return nil, errors.New("target-table is required when using stream-arn")
		}
		if *partitionKeyPtr == "" {
			return nil, errors.New("partition-key is required when using stream-arn")
		}
	}

	// Validate sample rate
	if *sampleRatePtr <= 0 {
		return nil, errors.New("sample-rate must be greater than 0")
	}

	return &CommandFlags{
		SourceProfile: *sourceProfilePtr,
		TargetProfile: *targetProfilePtr,
		StreamArn:     *streamArnPtr,
		TargetTable:   *targetTablePtr,
		PartitionKey:  *partitionKeyPtr,
		SortKey:       *sortKeyPtr,
		Region:        *regionPtr,
		SampleRate:    *sampleRatePtr,
	}, nil
}
