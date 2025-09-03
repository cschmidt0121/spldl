# spldl

A fast, concurrent Splunk search results downloader that allows you to quickly download results from large Splunk jobs.

## Installation

### Prerequisites

- Go 1.25.0 or later

```
go install github.com/cschmidt0121/spldl/cmd/spldl@latest
```

## Usage

### Basic Syntax

```
spldl [options] <output-file.[ndjson|csv|txt]>
```

The output file extension determines the format:
- `.ndjson` - Newline-delimited JSON
- `.csv` - CSV
- `.txt` - Raw events 

### Authentication

spldl supports two authentication methods:

#### Token Authentication (Recommended)
```bash
# Via command line
spldl --token "your-splunk-token" --host "splunk.example.com" --search "index=main | head 1000" results.ndjson

# Via environment variable
export SPLUNK_TOKEN="your-splunk-token"
spldl --host "splunk.example.com" --search "index=main | head 1000" results.ndjson
```

#### HTTP Basic Authentication
```bash
# Via command line
spldl --username "admin" --password "password" --host "splunk.example.com" --search "index=main | head 1000" results.ndjson

# Via environment variables
export SPLUNK_USERNAME="admin"
export SPLUNK_PASSWORD="password"
spldl --host "splunk.example.com" --search "index=main | head 1000" results.ndjson
```

### Examples

#### Execute a New Search
```bash
# Download last 24 hours of logs to JSON
spldl --token "your-token" --host "splunk.example.com" \
  --search "index=_internal | table _raw " \
  results.txt

# Search with custom time range
spldl --token "your-token" --host "splunk.example.com" \
  --search "index=main sourcetype=specific_st error | table _time src_ip error" \
  --earliest "-7d" --latest "now" \
  error_logs.csv

# Search and delete job when complete
spldl --token "your-token" --host "splunk.example.com" \
  --search "index=main | stats count by sourcetype" \
  --delete-when-done \
  stats.ndjson
```

#### Download from Existing Job ID
```bash
# Download results from a completed search job
spldl --token "your-token" --host "splunk.example.com" \
  --sid "1234567890.123" \
  existing_results.csv
```

## Concurrency warning

spldl opens multiple concurrent HTTP connections in order to download result sets quickly. By default, this is 8 connections. I have never observed degraded search head performance doing this, but if you are worried about limiting impact, you can lower the amount of concurrent connections by setting the `--max-connections` flag.


## Configuration Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--search` | - | - | Search query to execute |
| `--sid` | - | - | Existing search job ID to download |
| `--token` | `SPLUNK_TOKEN` | - | Splunk authentication token |
| `--username` | `SPLUNK_USERNAME` | - | Username for HTTP Basic auth |
| `--password` | `SPLUNK_PASSWORD` | - | Password for HTTP Basic auth |
| `--host` | - | - | Splunk server hostname |
| `--port` | - | `8089` | Splunk server port |
| `--earliest` | - | `-24h` | Earliest time for search |
| `--latest` | - | `now` | Latest time for search |
| `--max-connections` | - | `8` | Max concurrent download connections |
| `--delete-when-done`, `-d` | - | `false` | Delete job after download |
| `--insecure`, `-k` | - | `false` | Skip TLS certificate verification |
| `--help`, `-h` | - | - | Show help message |

## Limitations

- Maximum result limit: 500,000 events per job (see [Downloading multiple jobs](#downloading-multiple-jobs))
- All results must be on-disk on the target search head. **Use | table or another transforming command in order to guarantee this**. If you want to minimize disk usage, use the `--delete-when-done` flag.
- If using "raw" mode (.txt extension), make sure your events have a _raw field. It's a good idea to add `| table _raw` to your search as all other fields will be discarded anyway.

## Downloading multiple jobs

Because of the hard limit of 500,000 results per search job, downloading large time-ranges of data can be a challenge. I initially wanted this tool to automatically split a search into multiple jobs, but this is a deceptively difficult task due to the extreme expressiveness of SPL and potentially-inconsistent data ingest volume. 

Instead, I recommend the following:

1. Run multiple jobs, making sure each job has 500k results or less. 
2. For each job, extend their TTL. This is easily done by clicking the "Share" button. Then, note their SID (search ID).
3. Place each SID in a .txt file called sids.txt.
4. Tweak the following script with your environment/creds and run it.


### Bash (for *nix/MacOS users)
```bash
#!/bin/bash

SID_FILE="sids.txt"
TOKEN="your-token"
HOST="splunk.example.com"
OUTPUT_DIR="results"
FILE_EXTENSION="csv"

mkdir -p "$OUTPUT_DIR"

# Read SIDs from a file (one SID per line)
while IFS= read -r sid; do
    echo "Processing SID: $sid"
    spldl --token "$TOKEN" --host "$HOST" --sid "$sid" "$OUTPUT_DIR/results_${sid}.${FILE_EXTENSION}"
done < "$SID_FILE"
```

### PowerShell (For Windows users)

```powershell
# Configuration
$SID_FILE = "sids.txt"
$TOKEN = "your-token"
$HOST = "splunk.example.com"
$OUTPUT_DIR = "results"
$FILE_EXTENSION = "csv"

# Create output directory if it doesn't exist
New-Item -ItemType Directory -Force -Path $OUTPUT_DIR | Out-Null

# Read SIDs from file and process each one
Get-Content $SID_FILE | ForEach-Object {
    $sid = $_.Trim()
    if ($sid) {
        Write-Host "Processing SID: $sid"
        & spldl --token $TOKEN --host $HOST --sid $sid "$OUTPUT_DIR/results_$sid.$FILE_EXTENSION"
    }
}
```
## License

This project is licensed under the MIT License.
