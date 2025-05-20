# DynamoDB Migration Monitor

[English](#english) | [繁體中文](README_TW.md)

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A monitoring and validation tool for AWS DynamoDB table migration process. It monitors the DynamoDB Stream of the target table to track migration progress and perform sampling validation in real-time.

For technical details on the monitoring mechanism, see [Monitoring Mechanism](monitoring_mechanism.md).

## Monitoring Methods

Currently supported:
- Stream-based monitoring: Uses DynamoDB Streams to track changes in real-time

Coming soon:
- Table query-based monitoring: Will support direct table comparison through querying

## Migration Architecture

The migration architecture uses S3 export/import combined with DynamoDB Stream to achieve minimal downtime during data migration.

### Migration Timeline

```
T0 ─────────── T1 ─────────── T2 ─────────── T3 ─────────── T4
   ^            ^             ^              ^              ^
   |            |             |              |              |
   |            |             |              |              |
Enable Stream  Start S3    S3 Import    Replication     Validation
              Export      Complete      Complete        Complete

Timeline Details:
T0: Enable DynamoDB Stream, start tracking all data changes
T1: Start exporting data to S3, this is the baseline snapshot point
T2: S3 import complete, but changes during T1-T2 may not be synced yet
T3: Replication process complete, all changes during T1-T2 are synced
T4: Data validation complete, confirming all data is correctly migrated
```

### Migration Steps

1. **Enable DynamoDB Stream (T0)**
   - Enable DynamoDB Stream on source table
   - Set Stream type to `NEW_IMAGES`
   - Start recording all data changes

2. **Export Data to S3 (T1)**
   - Export table data from source account to S3
   - This is the baseline snapshot

3. **Import Data from S3 (T1 ~ T2)**
   - Import S3 data into target account's table
   - This process may take longer depending on data volume

4. **Data Replication (T1 ~ T3)**
   - DBA's replication script handles changes accumulated between T1 and T2
   - Ensures no data changes are lost during S3 export/import
   - Wait for replication script to consume all accumulated changes

5. **Setup Real-time Replication (T3)**
   - Confirm real-time replication mechanism is working
   - New changes should now sync in real-time

6. **Data Validation (T0 ~ T4)**
   - Use this tool for data validation
   - Validation strategy: Listen to source table changes, then query target table
   - This approach is more accurate than monitoring target table

```
Source Table ──── S3 Export ────┐
     │                         │
     │                         v
     │                   Target Table (S3 Import)
     │                         │
     └──── Stream ────────────┘
           (Real-time Replication)
```

### Validation Strategy

This tool employs the following validation approaches:

1. **Primary Strategy (Currently Used)**
   - Listen to source table's Stream
   - For each change, validate data by querying target table
   - Advantages:
     - Can accurately track all source data changes
     - No data changes will be missed
     - Can detect sync delays or failures in real-time
     - Suitable for long-running migration projects

2. **Alternative Strategy**
   - Listen to target table's Stream
   - For each change, validate data by querying source table
   - Disadvantages:
     - May not fully track all data changes
     - Cannot ensure all source table changes are replicated
     - More difficult to detect sync failures
     - Not suitable for migrations requiring high accuracy

### Why This Architecture?

1. **Minimize Downtime**
   - Use S3 for bulk data migration
   - Stream handles incremental updates during migration
   - No need to stop application writes

2. **Data Consistency Guarantee**
   - Stream ensures capture of all data changes
   - Replication script handles accumulated changes
   - Validation tool ensures data correctness

3. **Reliable Validation Mechanism**
   - Real-time monitoring of data changes
   - Supports sampling validation to reduce costs
   - Detailed validation statistics and logs

## Project Setup

1. Clone the repository:
```bash
git clone https://github.com/yotsuba1022/dynamodb-migration-monitor.git
cd dynamodb-migration-monitor
```

2. Download dependencies:
```bash
go mod download
```

3. Build the project:
```bash
go build
```

## Usage

### Basic Command Template

```bash
./dynamodb-migration-monitor \
  --source-profile <source_profile> \
  --target-profile <target_profile> \
  --stream-profile <stream_profile> \
  --stream-arn <stream_arn> \
  --target-table <table_name> \
  --partition-key <pk_name> \
  --region <aws_region>
```

### Complete Example

```bash
./dynamodb-migration-monitor \
  --source-profile source_profile \
  --target-profile target_profile \
  --stream-profile stream_profile \
  --stream-arn "arn:aws:dynamodb:ap-northeast-1:123456789012:table/my-table/stream/2024-01-01T00:00:00.000" \
  --target-table "my-table" \
  --partition-key "user_id" \
  --sort-key "timestamp" \
  --region ap-northeast-1 \
  --sample-rate 50 \
  --verify-on source \
  --verbose
```

### Parameters

Required:
- `--source-profile`: Source AWS profile name, used for accessing source table
- `--target-profile`: Target AWS profile name, used for accessing target table
- `--stream-arn`: Target table's Stream ARN, used for monitoring data changes
- `--target-table`: Target table name, the table to validate against
- `--partition-key`: Partition key name, used for data querying and comparison

Optional:
- `--stream-profile`: Stream AWS profile name (defaults to source profile). Use a dedicated profile if Stream access requires different permissions
- `--sort-key`: Sort key name (if table has one). Used for composite primary keys
- `--region`: AWS Region (default: ap-northeast-1). Specifies the AWS region to operate in
- `--sample-rate`: Validation sampling rate (default: 100). Can be reduced to lower costs, see sampling rate section for details
- `--verify-on`: Which table to verify against: source or target (default: source). Determines validation direction
- `--verbose`: Show success validation logs (default: false). When enabled, shows all validation details but may produce large output

## About Sampling Rate and Statistical Confidence

Even a **1% sampling rate** is statistically powerful for datasets of this magnitude. Consider a migration of **30 million records**:

| Aspect | 1% Sampling (300,000 samples) |
|--------|------------------------------|
| Absolute Sample Size | 300,000 is an extremely large sample—most academic studies rely on far fewer observations. |
| Margin of Error (95% CI) | ±0.18%. If the sample shows **99%** success, the true success rate is almost certainly between **98.82%** and **99.18%**. |
| Ability to Detect Rare Issues | A defect rate of **0.1%** still yields ≈ 300 error cases—enough for reliable detection and root-cause analysis. |
| Practical Considerations | Migration defects tend to be **systematic**, not random. Systematic issues will surface even at low sampling rates. If 1% passes, the likelihood of hidden systemic problems is extremely small. |

In short, a 1% sample offers an excellent balance between cost and accuracy. You can always increase the sampling rate if deeper inspection is required.

## Monitoring Output

Statistics are displayed every 30 seconds, including:
- Total and unique event counts
- INSERT and MODIFY operation counts
- Average events per second
- Validation success rate

## Prerequisites and Notes

1. AWS CLI Setup:
   - AWS CLI must be installed
   - Source and target AWS Profiles must be configured:
     ```bash
     # Check AWS Profile configuration
     aws configure list --profile <profile_name>
     
     # Verify DynamoDB access
     aws dynamodb list-tables --profile <profile_name>
     ```
   - For SSO login, ensure token is not expired:
     ```bash
     # SSO login
     aws sso login --profile <profile_name>
     ```

2. DynamoDB Stream Configuration:
   - Must be enabled with `NEW_IMAGES`
   - Verify stream configuration:
     ```bash
     aws dynamodb describe-table --table-name <table_name> --profile <profile_name> | grep StreamSpecification -A 5
     ```
   - Enable stream if needed:
     ```bash
     aws dynamodb update-table \
       --table-name <table_name> \
       --stream-specification StreamEnabled=true,StreamViewType=NEW_IMAGES \
       --profile <profile_name>
     ```

3. Required AWS Permissions:
   - Read access to source table
   - Stream read access to target table
   - Minimum required permissions:
     - `dynamodb:GetItem` (source table)
     - `dynamodb:DescribeTable`
     - `dynamodb:DescribeStream`
     - `dynamodb:GetRecords`
     - `dynamodb:GetShardIterator`
     - `dynamodb:ListStreams`

4. It's recommended to test with a small dataset before large-scale migration

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.