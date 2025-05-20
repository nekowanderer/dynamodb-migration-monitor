package internal

import (
	"errors"
	"flag"
)

// CommandFlags contains all command line parameters
type CommandFlags struct {
	SourceProfile string // Source AWS profile name
	TargetProfile string // Target AWS profile name
	StreamProfile string // Stream AWS profile name (optional, defaults to target profile)
	StreamArn     string // DynamoDB Stream ARN (optional)
	TargetTable   string // Target DynamoDB table name
	PartitionKey  string // Name of the partition key
	SortKey       string // Name of the sort key (optional)
	Region        string // AWS Region (optional, defaults to ap-northeast-1)
	SampleRate    int    // Validate 1 out of every SampleRate records (optional, defaults to 100)
	IteratorType  string // DynamoDB Stream Iterator Type (optional, defaults to LATEST)
	VerifyOn      string // Which table to verify against: source or target (optional, defaults to source)
	Verbose       bool   // Whether to show success validation logs (optional, defaults to false)
}

// ParseCommandFlags parses command line flags and returns the configuration
func ParseCommandFlags() (*CommandFlags, error) {
	sourceProfilePtr := flag.String("source-profile", "", "Source AWS profile name (required)")
	targetProfilePtr := flag.String("target-profile", "", "Target AWS profile name (required)")
	streamProfilePtr := flag.String("stream-profile", "", "Stream AWS profile name (optional, defaults to target profile)")
	streamArnPtr := flag.String("stream-arn", "", "DynamoDB Stream ARN for stream-based count check (optional)")
	targetTablePtr := flag.String("target-table", "", "Target DynamoDB table name for stream-based check (required if stream-arn is set)")
	partitionKeyPtr := flag.String("partition-key", "", "Name of the partition key (required if stream-arn is set)")
	sortKeyPtr := flag.String("sort-key", "", "Name of the sort key (optional)")
	regionPtr := flag.String("region", "ap-northeast-1", "AWS Region (optional, defaults to ap-northeast-1)")
	sampleRatePtr := flag.Int("sample-rate", 100, "Validate 1 out of every N records (optional, defaults to 100)")
	iteratorTypePtr := flag.String("iterator-type", "LATEST", "DynamoDB Stream Iterator Type (optional, LATEST or TRIM_HORIZON)")
	verifyOnPtr := flag.String("verify-on", "source", "Which table to verify against: source or target (optional, defaults to source)")
	verbosePtr := flag.Bool("verbose", false, "Show success validation logs (optional, defaults to false)")
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

	// Validate iterator type
	iteratorType := *iteratorTypePtr
	if iteratorType != "LATEST" && iteratorType != "TRIM_HORIZON" {
		return nil, errors.New("iterator-type must be either LATEST or TRIM_HORIZON")
	}

	// Validate verify-on
	verifyOn := *verifyOnPtr
	if verifyOn != "source" && verifyOn != "target" {
		return nil, errors.New("verify-on must be either source or target")
	}

	// If stream-profile is not set, use source-profile
	streamProfile := *streamProfilePtr
	if streamProfile == "" {
		streamProfile = *sourceProfilePtr
	}

	return &CommandFlags{
		SourceProfile: *sourceProfilePtr,
		TargetProfile: *targetProfilePtr,
		StreamProfile: streamProfile,
		StreamArn:     *streamArnPtr,
		TargetTable:   *targetTablePtr,
		PartitionKey:  *partitionKeyPtr,
		SortKey:       *sortKeyPtr,
		Region:        *regionPtr,
		SampleRate:    *sampleRatePtr,
		IteratorType:  iteratorType,
		VerifyOn:      verifyOn,
		Verbose:       *verbosePtr,
	}, nil
}
