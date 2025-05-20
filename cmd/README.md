# DynamoDB Testing Tools

This directory contains two utility tools to help test the DynamoDB Migration Monitor:

## datagen - Data Generator

The `datagen` tool quickly generates test data in DynamoDB and records all generated key pairs to a CSV file for later deletion.

### Features

- Generate a specified number of test data records
- Choose between single-item or batch writes
- Control write interval for stream processing testing
- Record generated key pairs to a CSV file
- Customize partition key and sort key names

### Usage

```bash
go run ./cmd/datagen/main.go \
  --profile <AWS_PROFILE> \
  --table <TABLE_NAME> \
  --count <DATA_COUNT> \
  --partition-key <PARTITION_KEY_NAME> \
  --sort-key <SORT_KEY_NAME> \
  --region <AWS_REGION> \
  --wait <WRITE_INTERVAL_MS> \
  --output <OUTPUT_CSV_FILE> \
  --batch
```

### Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `--profile` | Yes | None | AWS profile name |
| `--table` | Yes | None | DynamoDB table name |
| `--count` | No | 10 | Number of items to generate |
| `--partition-key` | No | pk | Partition key name |
| `--sort-key` | No | sk | Sort key name |
| `--region` | No | ap-northeast-1 | AWS region |
| `--wait` | No | 0 | Write interval in milliseconds (single mode only) |
| `--output` | No | None | Output CSV file path to record all generated key pairs |
| `--batch` | No | false | Enable batch write mode |

### Examples

```bash
# Generate 100 test items with 100ms interval between writes
go run ./cmd/datagen/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --count 100 \
  --partition-key pk \
  --sort-key sk \
  --region ap-northeast-1 \
  --wait 100 \
  --output ./test_keys.csv

# Batch write 1000 items
go run ./cmd/datagen/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --count 1000 \
  --batch \
  --output ./test_keys.csv
```

---

## datadel - Data Deletion Tool

The `datadel` tool reads key pairs from a CSV file and deletes the corresponding items from a DynamoDB table.

### Features

- Read key pairs from a CSV file
- Choose between single-item or batch deletions
- Dry run mode (no actual deletions)
- Verbose output mode
- Customize partition key and sort key names

### Usage

```bash
go run ./cmd/datadel/main.go \
  --profile <AWS_PROFILE> \
  --table <TABLE_NAME> \
  --input <CSV_FILE> \
  --partition-key <PARTITION_KEY_NAME> \
  --sort-key <SORT_KEY_NAME> \
  --region <AWS_REGION> \
  --wait <DELETE_INTERVAL_MS> \
  --batch \
  --dry-run \
  --verbose
```

### Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `--profile` | Yes | None | AWS profile name |
| `--table` | Yes | None | DynamoDB table name |
| `--input` | Yes | None | CSV file path containing key pairs to delete |
| `--partition-key` | No | pk | Partition key name |
| `--sort-key` | No | sk | Sort key name |
| `--region` | No | ap-northeast-1 | AWS region |
| `--wait` | No | 0 | Delete interval in milliseconds (single mode only) |
| `--batch` | No | false | Enable batch delete mode |
| `--dry-run` | No | false | Dry run mode, no actual deletions |
| `--verbose` | No | false | Verbose output mode |

### Examples

```bash
# Dry run mode to see what would be deleted
go run ./cmd/datadel/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --dry-run \
  --verbose

# Actually delete data using batch mode
go run ./cmd/datadel/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --batch
```

---

## Complete Testing Workflow Example

Here is a demonstration of a complete testing workflow:

1. Start the DynamoDB Migration Monitor:

```bash
go run . \
  --source-profile 362395300803_dev \
  --target-profile 362395300803_dev \
  --stream-profile 362395300803_dev \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:362395300803:table/test_ddb/stream/2025-05-20T13:05:28.798" \
  --target-table "test_ddb" \
  --partition-key "pk" \
  --sort-key "sk" \
  --region ap-northeast-1 \
  --sample-rate 1 \
  --verify-on source \
  --iterator-type TRIM_HORIZON \
  --verbose true
```

2. In another terminal window, generate test data:

```bash
go run ./cmd/datagen/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --count 100 \
  --wait 100 \
  --output ./test_keys.csv
```

3. Observe if the Monitor correctly processes the stream events

4. Clean up test data after testing is complete:

```bash
go run ./cmd/datadel/main.go \
  --profile 362395300803_dev \
  --table test_ddb \
  --input ./test_keys.csv \
  --batch
``` 