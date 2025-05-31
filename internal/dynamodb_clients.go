package internal

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	log "github.com/sirupsen/logrus"
)

// ClientConfig holds AWS profile configuration for creating DynamoDB clients
type ClientConfig struct {
	SourceProfile string
	TargetProfile string
	StreamProfile string // Optional, profile for Stream client (defaults to SourceProfile)
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
		cfg.Region = "ap-northeast-1"
	}

	// Set default StreamProfile if not provided
	streamProfile := cfg.StreamProfile
	if streamProfile == "" {
		streamProfile = cfg.SourceProfile
		log.Infof("No Stream profile specified, using Source profile: %s", streamProfile)
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

	// Use the specified stream profile
	streamClient, err := NewDynamoDBStreamClient(ctx, streamProfile, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB Stream client for profile %s: %w", streamProfile, err)
	}

	return &DynamoDBClients{
		SourceClient: sourceClient,
		TargetClient: targetClient,
		StreamClient: streamClient,
	}, nil
}

// NewDynamoDBClient creates a new DynamoDB client with the specified profile
func NewDynamoDBClient(ctx context.Context, profile, region string) (*dynamodb.Client, error) {
	var cfg aws.Config
	var err error

	// try to use profile
	if profile != "" {
		// use profile
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithSharedConfigProfile(profile),
			config.WithRegion(region),
		)

		if err == nil {
			// successfully load profile
			_, err = cfg.Credentials.Retrieve(ctx)
			if err == nil {
				log.Infof("✅ Successfully loaded credentials for profile %s", profile)
				return dynamodb.NewFromConfig(cfg), nil
			}
			log.Warnf("Failed to use profile %s: %v", profile, err)
		}
	}

	// if no profile or profile is not available, try to use EC2 IAM role
	log.Infof("Attempting to use EC2 instance role")
	cfg, err = config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load EC2 instance role: %w", err)
	}

	// verify credentials
	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve EC2 role credentials: %w", err)
	}

	log.Infof("✅ Successfully loaded EC2 instance role credentials")
	return dynamodb.NewFromConfig(cfg), nil
}

// NewDynamoDBStreamClient creates a new DynamoDB Streams client with the specified profile
func NewDynamoDBStreamClient(ctx context.Context, profile, region string) (*dynamodbstreams.Client, error) {
	var cfg aws.Config
	var err error

	// try to use profile
	if profile != "" {
		// use profile
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithSharedConfigProfile(profile),
			config.WithRegion(region),
		)

		if err == nil {
			// successfully load profile
			_, err = cfg.Credentials.Retrieve(ctx)
			if err == nil {
				log.Infof("✅ Successfully loaded credentials for profile %s", profile)
				return dynamodbstreams.NewFromConfig(cfg), nil
			}
			log.Warnf("Failed to use profile %s: %v", profile, err)
		}
	}

	// if no profile or profile is not available, try to use EC2 IAM role
	log.Infof("Attempting to use EC2 instance role")
	cfg, err = config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load EC2 instance role: %w", err)
	}

	// verify credentials
	_, err = cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve EC2 role credentials: %w", err)
	}

	log.Infof("✅ Successfully loaded EC2 instance role credentials")
	return dynamodbstreams.NewFromConfig(cfg), nil
}
