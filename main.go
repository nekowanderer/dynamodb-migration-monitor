package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/yotsuba1022/dynamodb-migration-monitor/internal"

	log "github.com/sirupsen/logrus"
)

func main() {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	// Parse command line flags
	cmdFlags, err := internal.ParseCommandFlags()
	if err != nil {
		log.Fatalf("Error parsing command line flags: %v", err)
	}

	// Create DynamoDB clients
	clients, err := internal.NewDynamoDBClients(ctx, internal.ClientConfig{
		SourceProfile: cmdFlags.SourceProfile,
		TargetProfile: cmdFlags.TargetProfile,
		Region:        cmdFlags.Region,
	})
	if err != nil {
		log.Fatalf("Failed to create DynamoDB clients: %v", err)
	}

	// Set up signal handling
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Listen for signals
	go func() {
		select {
		case <-c:
			log.Info("Received interrupt signal, initiating shutdown...")
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	// Run the stream-based verification if the streamArn is provided
	if cmdFlags.StreamArn != "" {
		// Run the stream-based verification process
		internal.RunStreamStyleVerification(ctx, &internal.StreamVerificationConfig{
			SourceClient: clients.SourceClient,
			TargetClient: clients.TargetClient,
			StreamClient: clients.StreamClient,
			StreamArn:    cmdFlags.StreamArn,
			TargetTable:  cmdFlags.TargetTable,
			SampleRate:   cmdFlags.SampleRate,
			PartitionKey: cmdFlags.PartitionKey,
			SortKey:      cmdFlags.SortKey,
		})
		return
	}
}
