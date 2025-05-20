package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DataGenConfig struct {
	Profile      string
	Region       string
	TableName    string
	ItemCount    int
	PartitionKey string
	SortKey      string
	Batch        bool
	WaitTime     int
	OutputFile   string
}

func main() {
	// Parse command line flags
	cfg := parseFlags()

	// Set up AWS config and DynamoDB client
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithSharedConfigProfile(cfg.Profile),
	)
	if err != nil {
		log.Fatalf("Unable to load AWS config: %v", err)
	}

	client := dynamodb.NewFromConfig(awsCfg)

	// Prepare output file for keys
	var keysFile *os.File
	if cfg.OutputFile != "" {
		// Create directory if it doesn't exist
		dir := filepath.Dir(cfg.OutputFile)
		if dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatalf("Failed to create directory for output file: %v", err)
			}
		}

		// Open output file
		keysFile, err = os.Create(cfg.OutputFile)
		if err != nil {
			log.Fatalf("Failed to create output file: %v", err)
		}
		defer keysFile.Close()

		// Write header
		_, err = keysFile.WriteString(fmt.Sprintf("%s,%s\n", cfg.PartitionKey, cfg.SortKey))
		if err != nil {
			log.Fatalf("Failed to write header to output file: %v", err)
		}
	}

	// Generate and insert data
	var keys []string
	if cfg.Batch {
		keys = generateBatchData(client, cfg)
	} else {
		keys = generateSingleData(client, cfg)
	}

	// Write keys to file
	if keysFile != nil && len(keys) > 0 {
		_, err = keysFile.WriteString(strings.Join(keys, "\n"))
		if err != nil {
			log.Fatalf("Failed to write keys to output file: %v", err)
		}
		log.Printf("Successfully saved %d keys to %s", len(keys), cfg.OutputFile)
	}

	log.Printf("Successfully generated %d items in table %s", cfg.ItemCount, cfg.TableName)
}

func parseFlags() *DataGenConfig {
	cfg := &DataGenConfig{}

	flag.StringVar(&cfg.Profile, "profile", "", "AWS profile to use")
	flag.StringVar(&cfg.Region, "region", "ap-northeast-1", "AWS region")
	flag.StringVar(&cfg.TableName, "table", "", "DynamoDB table name")
	flag.IntVar(&cfg.ItemCount, "count", 10, "Number of items to generate")
	flag.StringVar(&cfg.PartitionKey, "partition-key", "pk", "Partition key name")
	flag.StringVar(&cfg.SortKey, "sort-key", "sk", "Sort key name")
	flag.BoolVar(&cfg.Batch, "batch", false, "Use batch write")
	flag.IntVar(&cfg.WaitTime, "wait", 0, "Time to wait between writes in milliseconds (single mode only)")
	flag.StringVar(&cfg.OutputFile, "output", "", "File to save generated keys (CSV format)")

	flag.Parse()

	if cfg.Profile == "" {
		log.Fatal("AWS profile is required")
	}
	if cfg.TableName == "" {
		log.Fatal("Table name is required")
	}

	return cfg
}

func generateSingleData(client *dynamodb.Client, cfg *DataGenConfig) []string {
	var keys []string

	for i := 1; i <= cfg.ItemCount; i++ {
		// Create a random item
		item := createRandomItem(i, cfg)

		// Extract keys
		pk := item[cfg.PartitionKey].(*types.AttributeValueMemberS).Value
		sk := item[cfg.SortKey].(*types.AttributeValueMemberS).Value
		keys = append(keys, fmt.Sprintf("%s,%s", pk, sk))

		// Put the item in DynamoDB
		_, err := client.PutItem(context.TODO(), &dynamodb.PutItemInput{
			TableName: aws.String(cfg.TableName),
			Item:      item,
		})
		if err != nil {
			log.Fatalf("Failed to put item %d: %v", i, err)
		}

		log.Printf("Added item %d: %s,%s", i, pk, sk)

		// Wait if configured
		if cfg.WaitTime > 0 {
			time.Sleep(time.Duration(cfg.WaitTime) * time.Millisecond)
		}
	}

	return keys
}

func generateBatchData(client *dynamodb.Client, cfg *DataGenConfig) []string {
	var keys []string
	const maxBatchSize = 25
	batchSize := min(maxBatchSize, cfg.ItemCount)

	for i := 0; i < cfg.ItemCount; i += batchSize {
		// Prepare batch request
		var writeRequests []types.WriteRequest
		var batchKeys []string

		// Calculate how many items to write in this batch
		currentBatchSize := min(batchSize, cfg.ItemCount-i)

		for j := 0; j < currentBatchSize; j++ {
			itemNum := i + j + 1
			item := createRandomItem(itemNum, cfg)

			// Extract keys
			pk := item[cfg.PartitionKey].(*types.AttributeValueMemberS).Value
			sk := item[cfg.SortKey].(*types.AttributeValueMemberS).Value
			batchKeys = append(batchKeys, fmt.Sprintf("%s,%s", pk, sk))

			writeRequests = append(writeRequests, types.WriteRequest{
				PutRequest: &types.PutRequest{
					Item: item,
				},
			})
		}

		// Execute batch write
		_, err := client.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				cfg.TableName: writeRequests,
			},
		})
		if err != nil {
			log.Fatalf("Failed to batch write items %d to %d: %v", i+1, i+currentBatchSize, err)
		}

		keys = append(keys, batchKeys...)
		log.Printf("Added items %d to %d", i+1, i+currentBatchSize)
	}

	return keys
}

func createRandomItem(num int, cfg *DataGenConfig) map[string]types.AttributeValue {
	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(num)))

	// Create a basic item with partition and sort keys
	item := map[string]types.AttributeValue{
		cfg.PartitionKey: &types.AttributeValueMemberS{
			Value: fmt.Sprintf("TEST_PK_%d", num),
		},
		cfg.SortKey: &types.AttributeValueMemberS{
			Value: fmt.Sprintf("TEST_SK_%d", num),
		},
		"id": &types.AttributeValueMemberN{
			Value: strconv.Itoa(num),
		},
		"timestamp": &types.AttributeValueMemberS{
			Value: time.Now().Format(time.RFC3339),
		},
		"randomValue": &types.AttributeValueMemberN{
			Value: strconv.Itoa(r.Intn(1000)),
		},
		"data": &types.AttributeValueMemberS{
			Value: fmt.Sprintf("This is test data #%d", num),
		},
	}

	return item
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
