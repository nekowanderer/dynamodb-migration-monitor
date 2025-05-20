package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DataDelConfig struct {
	Profile      string
	Region       string
	TableName    string
	InputFile    string
	Batch        bool
	WaitTime     int
	DryRun       bool
	Verbose      bool
	PartitionKey string
	SortKey      string
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

	// Read keys from input file
	keyPairs, err := readKeysFromFile(cfg.InputFile)
	if err != nil {
		log.Fatalf("Failed to read keys from file: %v", err)
	}

	log.Printf("Found %d items to delete from file %s", len(keyPairs), cfg.InputFile)
	if cfg.DryRun {
		log.Printf("DRY RUN: No items will be deleted")
	}

	// Delete items
	deleted := 0
	if cfg.Batch {
		deleted = deleteBatchItems(client, cfg, keyPairs)
	} else {
		deleted = deleteSingleItems(client, cfg, keyPairs)
	}

	log.Printf("Successfully deleted %d/%d items from table %s", deleted, len(keyPairs), cfg.TableName)
}

func parseFlags() *DataDelConfig {
	cfg := &DataDelConfig{}

	flag.StringVar(&cfg.Profile, "profile", "", "AWS profile to use")
	flag.StringVar(&cfg.Region, "region", "ap-northeast-1", "AWS region")
	flag.StringVar(&cfg.TableName, "table", "", "DynamoDB table name")
	flag.StringVar(&cfg.InputFile, "input", "", "CSV file with keys to delete")
	flag.BoolVar(&cfg.Batch, "batch", false, "Use batch delete")
	flag.IntVar(&cfg.WaitTime, "wait", 0, "Time to wait between deletes in milliseconds (single mode only)")
	flag.BoolVar(&cfg.DryRun, "dry-run", false, "Dry run (don't actually delete items)")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	flag.StringVar(&cfg.PartitionKey, "partition-key", "pk", "Partition key name")
	flag.StringVar(&cfg.SortKey, "sort-key", "sk", "Sort key name")

	flag.Parse()

	if cfg.Profile == "" {
		log.Fatal("AWS profile is required")
	}
	if cfg.TableName == "" {
		log.Fatal("Table name is required")
	}
	if cfg.InputFile == "" {
		log.Fatal("Input file is required")
	}

	return cfg
}

// Read keys from CSV file
// Format: pk,sk
func readKeysFromFile(filePath string) ([][]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 2
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Check for common header names for partition and sort keys
	if !isHeaderRow(header) {
		// If not a header, reopen the file to start from the beginning
		file.Close()
		file, err = os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to reopen file: %w", err)
		}
		defer file.Close()
		reader = csv.NewReader(file)
		reader.FieldsPerRecord = 2
		reader.TrimLeadingSpace = true
	}

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read records: %w", err)
	}

	return records, nil
}

// Check if a line is likely a header row
func isHeaderRow(row []string) bool {
	commonPKNames := []string{"pk", "partitionkey", "partition_key", "id", "hash"}
	commonSKNames := []string{"sk", "sortkey", "sort_key", "range", "range_key"}

	pkMatch := false
	for _, name := range commonPKNames {
		if strings.EqualFold(strings.TrimSpace(row[0]), name) {
			pkMatch = true
			break
		}
	}

	skMatch := false
	for _, name := range commonSKNames {
		if strings.EqualFold(strings.TrimSpace(row[1]), name) {
			skMatch = true
			break
		}
	}

	return pkMatch && skMatch
}

func deleteSingleItems(client *dynamodb.Client, cfg *DataDelConfig, keyPairs [][]string) int {
	deleted := 0

	for i, keyPair := range keyPairs {
		if len(keyPair) != 2 {
			log.Printf("WARNING: Skipping invalid key pair at line %d: %v", i+1, keyPair)
			continue
		}

		pk := keyPair[0]
		sk := keyPair[1]

		key := map[string]types.AttributeValue{
			cfg.PartitionKey: &types.AttributeValueMemberS{Value: pk},
			cfg.SortKey:      &types.AttributeValueMemberS{Value: sk},
		}

		if cfg.Verbose {
			log.Printf("Deleting item %d: %s,%s", i+1, pk, sk)
		}

		if !cfg.DryRun {
			_, err := client.DeleteItem(context.TODO(), &dynamodb.DeleteItemInput{
				TableName: aws.String(cfg.TableName),
				Key:       key,
			})
			if err != nil {
				log.Printf("Failed to delete item %d: %v", i+1, err)
				continue
			}
		}

		deleted++

		// Wait if configured
		if cfg.WaitTime > 0 {
			time.Sleep(time.Duration(cfg.WaitTime) * time.Millisecond)
		}
	}

	return deleted
}

func deleteBatchItems(client *dynamodb.Client, cfg *DataDelConfig, keyPairs [][]string) int {
	deleted := 0
	const maxBatchSize = 25
	batchSize := min(maxBatchSize, len(keyPairs))

	for i := 0; i < len(keyPairs); i += batchSize {
		// Prepare batch request
		var writeRequests []types.WriteRequest

		// Calculate how many items to delete in this batch
		currentBatchSize := min(batchSize, len(keyPairs)-i)

		for j := 0; j < currentBatchSize; j++ {
			itemIdx := i + j
			keyPair := keyPairs[itemIdx]

			if len(keyPair) != 2 {
				log.Printf("WARNING: Skipping invalid key pair at line %d: %v", itemIdx+1, keyPair)
				continue
			}

			pk := keyPair[0]
			sk := keyPair[1]

			key := map[string]types.AttributeValue{
				cfg.PartitionKey: &types.AttributeValueMemberS{Value: pk},
				cfg.SortKey:      &types.AttributeValueMemberS{Value: sk},
			}

			writeRequests = append(writeRequests, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: key,
				},
			})
		}

		if cfg.Verbose {
			log.Printf("Deleting items %d to %d", i+1, i+len(writeRequests))
		}

		if !cfg.DryRun && len(writeRequests) > 0 {
			// Execute batch delete
			_, err := client.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					cfg.TableName: writeRequests,
				},
			})
			if err != nil {
				log.Printf("Failed to batch delete items %d to %d: %v", i+1, i+len(writeRequests), err)
				continue
			}
			deleted += len(writeRequests)
		} else if cfg.DryRun {
			deleted += len(writeRequests)
		}
	}

	return deleted
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
