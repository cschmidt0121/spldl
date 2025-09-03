package downloader

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/cschmidt0121/spldl/internal/config"
	"github.com/cschmidt0121/spldl/internal/splunkclient"
)

func createTestClient(testServerURL string, outputMode string) *splunkclient.Client {
	testURL, _ := url.Parse(testServerURL)

	host := strings.Split(testURL.Host, ":")[0]
	port := 80
	if testURL.Port() != "" {
		if p, err := strconv.Atoi(testURL.Port()); err == nil {
			port = p
		}
	}

	testConfig := config.ClientConfig{
		Host:      host,
		Port:      port,
		UseTLS:    false,
		VerifyTLS: false,
		Auth: config.AuthConfig{
			Type:     config.AuthHTTPBasic,
			Username: "testuser",
			Password: "testpass",
		},
	}

	return splunkclient.NewClient(testConfig)
}

func TestDownloadSearchResults(t *testing.T) {
	jsonData, err := os.ReadFile("testdata/results.json")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	csvData, err := os.ReadFile("testdata/results.csv")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	rawData, err := os.ReadFile("testdata/results.raw")
	if err != nil {
		t.Fatalf("Failed to read test data: %v", err)
	}

	jobStatusData, err := os.ReadFile("testdata/job_status.json")
	if err != nil {
		t.Fatalf("Failed to read job status test data: %v", err)
	}

	tests := []struct {
		name               string
		outputMode         string
		expectedData       []byte
		sid                string
		deleteWhenDone     bool
		simulateIncomplete bool
		simulateFailed     bool
		simulateTooMany    bool
		shouldError        bool
		expectedError      string
	}{
		{
			name:           "successful download with JSON output",
			outputMode:     "json",
			expectedData:   jsonData,
			sid:            "1756172871.1180",
			deleteWhenDone: false,
			shouldError:    false,
		},
		{
			name:           "successful download with CSV output",
			outputMode:     "csv",
			expectedData:   csvData,
			sid:            "1756172871.1180",
			deleteWhenDone: false,
			shouldError:    false,
		},
		{
			name:           "successful download with raw output",
			outputMode:     "raw",
			expectedData:   rawData,
			sid:            "1756172871.1180",
			deleteWhenDone: false,
			shouldError:    false,
		},
		{
			name:           "successful download with delete when done",
			outputMode:     "json",
			expectedData:   jsonData,
			sid:            "1756172871.1180",
			deleteWhenDone: true,
			shouldError:    false,
		},
		{
			name:               "job not complete error",
			outputMode:         "json",
			sid:                "1756172871.1180",
			simulateIncomplete: true,
			shouldError:        true,
			expectedError:      "is not complete",
		},
		{
			name:           "failed job error",
			outputMode:     "json",
			sid:            "1756172871.1180",
			simulateFailed: true,
			shouldError:    true,
			expectedError:  "has failed",
		},
		{
			name:            "too many results error",
			outputMode:      "json",
			sid:             "1756172871.1180",
			simulateTooMany: true,
			shouldError:     true,
			expectedError:   "more than 500000 results",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile, err := os.CreateTemp("", "test_download_*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())
			tempFile.Close()

			var jobStatusCalled, resultsCalled, deleteCalled bool
			resultCallCount := 0

			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle job delete requests
				if r.Method == "DELETE" && r.URL.Path == "/services/search/v2/jobs/"+tt.sid {
					deleteCalled = true
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("{}"))
					return
				}
				// Handle job status requests
				if r.Method == "GET" && r.URL.Path == "/services/search/v2/jobs/"+tt.sid {
					jobStatusCalled = true

					// Modify job status based on test scenario
					var jobStatus map[string]interface{}
					json.Unmarshal(jobStatusData, &jobStatus)

					if tt.simulateIncomplete {
						entry := jobStatus["entry"].([]interface{})[0].(map[string]interface{})
						content := entry["content"].(map[string]interface{})
						content["isDone"] = false
						content["dispatchState"] = "RUNNING"
						content["doneProgress"] = 0.5
					}

					if tt.simulateFailed {
						entry := jobStatus["entry"].([]interface{})[0].(map[string]interface{})
						content := entry["content"].(map[string]interface{})
						content["isFailed"] = true
					}

					if tt.simulateTooMany {
						entry := jobStatus["entry"].([]interface{})[0].(map[string]interface{})
						content := entry["content"].(map[string]interface{})
						content["resultCount"] = 600000
					}

					modifiedData, _ := json.Marshal(jobStatus)
					w.WriteHeader(http.StatusOK)
					w.Write(modifiedData)
					return
				}

				// Handle results requests
				if r.URL.Path == "/services/search/v2/jobs/"+tt.sid+"/results" {
					resultsCalled = true
					resultCallCount++

					outputMode := r.URL.Query().Get("output_mode")
					if outputMode != tt.outputMode {
						t.Errorf("Expected output_mode=%s, got %s", tt.outputMode, outputMode)
					}

					var mockResponse []byte
					switch outputMode {
					case "json":
						mockResponse = jsonData
					case "csv":
						mockResponse = csvData
					case "raw":
						mockResponse = rawData
					default:
						mockResponse = jsonData
					}

					w.WriteHeader(http.StatusOK)
					w.Write(mockResponse)
					return
				}

				// Everything else
				t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}))
			defer testServer.Close()

			client := createTestClient(testServer.URL, tt.outputMode)

			downloader := NewDownloader(client, config.DownloaderConfig{
				OutputMode:     tt.outputMode,
				DeleteWhenDone: tt.deleteWhenDone,
				MaxConnections: 8,
				SID:            tt.sid,
				Filename:       tempFile.Name(),
			})
			err = downloader.DownloadSearchResults()

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Expected no error, got %v", err)
				return
			}

			if !jobStatusCalled {
				t.Error("Expected job status to be called")
			}

			if !tt.shouldError {
				if !resultsCalled {
					t.Error("Expected results to be called")
				}

				if tt.deleteWhenDone && !deleteCalled {
					t.Error("Expected delete to be called when deleteWhenDone=true")
				}
				if !tt.deleteWhenDone && deleteCalled {
					t.Error("Expected delete NOT to be called when deleteWhenDone=false")
				}

				// Verify file was created and has expected content structure
				if _, err := os.Stat(tempFile.Name()); os.IsNotExist(err) {
					t.Error("Expected output file to be created")
				} else {
					fileContent, err := os.ReadFile(tempFile.Name())
					if err != nil {
						t.Errorf("Failed to read output file: %v", err)
					}
					if len(fileContent) == 0 {
						t.Error("Expected output file to have content")
					}

					// For JSON mode, verify it contains processed JSON lines
					if tt.outputMode == "json" {
						var js interface{}
						for _, line := range strings.Split(string(fileContent), "\n") {
							if line == "" {
								continue
							}
							if err := json.Unmarshal([]byte(line), &js); err != nil {
								t.Errorf("Output file content is not valid JSON: %v\nContent: %s", err, line)
							}
						}
					}
					// For CSV mode, verify it is a proper CSV file
					if tt.outputMode == "csv" {
						r := strings.NewReader(string(fileContent))
						_, err := csv.NewReader(r).ReadAll()
						if err != nil {
							t.Errorf("Output file content is not valid CSV: %v\nContent: %s", err, string(fileContent))
						}
					}

					// Finally, just check raw byte-for-byte
					if tt.outputMode == "raw" {
						// Verify the file contains the expected raw data
						if !bytes.Equal(fileContent, rawData) {
							t.Errorf("Output file content does not match expected raw data")
						}
					}
				}
			}
		})
	}
}
