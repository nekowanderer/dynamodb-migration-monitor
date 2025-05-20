package internal

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	log "github.com/sirupsen/logrus"
)

// ClientConfig holds AWS profile configuration for creating DynamoDB clients
type ClientConfig struct {
	SourceProfile string
	TargetProfile string
	StreamProfile string // Optional, defaults to TargetProfile
	Region        string // Optional, defaults to ap-southeast-1
}

// DynamoDBClients holds all necessary DynamoDB clients
type DynamoDBClients struct {
	SourceClient *dynamodb.Client
	TargetClient *dynamodb.Client
	StreamClient *dynamodbstreams.Client
}

// NewDynamoDBClients creates all required DynamoDB clients
func NewDynamoDBClients(ctx context.Context, cfg ClientConfig) (*DynamoDBClients, error) {
	// Set default region if not provided
	if cfg.Region == "" {
		cfg.Region = "ap-southeast-1"
	}

	// Set default stream profile if not provided
	if cfg.StreamProfile == "" {
		cfg.StreamProfile = cfg.TargetProfile
	}

	sourceClient, err := NewDynamoDBClient(ctx, cfg.SourceProfile, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB client for source profile: %w", err)
	}

	// Configure the target Dynamodb client
	targetClient, err := NewDynamoDBClient(ctx, cfg.TargetProfile, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB client for target profile: %w", err)
	}

	// Use stream profile for stream client
	streamClient, err := NewDynamoDBStreamClient(ctx, cfg.StreamProfile, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB Stream client for stream profile: %w", err)
	}

	return &DynamoDBClients{
		SourceClient: sourceClient,
		TargetClient: targetClient,
		StreamClient: streamClient,
	}, nil
}

// NewDynamoDBClient creates a new DynamoDB client with the specified profile
func NewDynamoDBClient(ctx context.Context, profile, region string) (*dynamodb.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load configuration for profile %q: %w", profile, err)
	}

	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials for profile %q: %w", profile, err)
	}

	log.Infof("✅ Successfully loaded credentials for profile %s", profile)
	return dynamodb.NewFromConfig(cfg), nil
}

// NewDynamoDBStreamClient creates a new DynamoDB Streams client with the specified profile
func NewDynamoDBStreamClient(ctx context.Context, profile, region string) (*dynamodbstreams.Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithSharedConfigProfile(profile),
		config.WithRegion(region),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to load configuration for profile %q: %w", profile, err)
	}

	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials for profile %q: %w", profile, err)
	}

	log.Infof("✅ Successfully loaded credentials for profile %s", profile)
	return dynamodbstreams.NewFromConfig(cfg), nil
}
