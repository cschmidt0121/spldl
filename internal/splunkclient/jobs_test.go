package splunkclient

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/cschmidt0121/spldl/internal/config"
)

func assertJobContentEqual(t *testing.T, expected, actual SearchJobContent) {
	t.Helper()

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("JobStatus mismatch:\nExpected: %+v\nGot:      %+v", expected, actual)

		// Provide detailed field-by-field comparison for easier debugging
		if actual.SID != expected.SID {
			t.Errorf("  SID: expected %q, got %q", expected.SID, actual.SID)
		}
		if actual.ResultCount != expected.ResultCount {
			t.Errorf("  ResultCount: expected %d, got %d", expected.ResultCount, actual.ResultCount)
		}
		if actual.IsDone != expected.IsDone {
			t.Errorf("  IsDone: expected %t, got %t", expected.IsDone, actual.IsDone)
		}
		if actual.IsFailed != expected.IsFailed {
			t.Errorf("  IsFailed: expected %t, got %t", expected.IsFailed, actual.IsFailed)
		}
		if actual.DispatchState != expected.DispatchState {
			t.Errorf("  DispatchState: expected %q, got %q", expected.DispatchState, actual.DispatchState)
		}
		if actual.DoneProgress != expected.DoneProgress {
			t.Errorf("  DoneProgress: expected %f, got %f", expected.DoneProgress, actual.DoneProgress)
		}
		if !actual.EarliestTime.Equal(expected.EarliestTime) {
			t.Errorf("  EarliestTime: expected %s, got %s", expected.EarliestTime, actual.EarliestTime)
		}
		if !actual.LatestTime.Equal(expected.LatestTime) {
			t.Errorf("  LatestTime: expected %s, got %s", expected.LatestTime, actual.LatestTime)
		}
		if actual.EventCount != expected.EventCount {
			t.Errorf("  EventCount: expected %d, got %d", expected.EventCount, actual.EventCount)
		}
		if actual.EventAvailableCount != expected.EventAvailableCount {
			t.Errorf("  EventAvailableCount: expected %d, got %d", expected.EventAvailableCount, actual.EventAvailableCount)
		}
		if actual.RunDuration != expected.RunDuration {
			t.Errorf("  RunDuration: expected %f, got %f", expected.RunDuration, actual.RunDuration)
		}
	}
}

func TestJobStatus(t *testing.T) {
	// Create a test server that returns a mock Splunk job status response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request path and query parameters
		expectedPath := "/services/search/v2/jobs/1756064805.1039"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		outputMode := r.URL.Query().Get("output_mode")
		if outputMode != "json" {
			t.Errorf("Expected output_mode=json, got %s", outputMode)
		}

		data, err := os.ReadFile("testdata/job_status.json")
		if err != nil {
			t.Fatalf("Failed to read test data: %v", err)
		}

		// Use data in your test
		mockResponse := string(data)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer testServer.Close()

	testConfig := config.ClientConfig{
		Host:      "localhost",
		Port:      8089,
		UseTLS:    false,
		VerifyTLS: false,
		Auth: config.AuthConfig{
			Type:     config.AuthHTTPBasic,
			Username: "testuser",
			Password: "testpass",
		},
	}

	client := NewClient(testConfig)

	client.baseURL = testServer.URL

	sid := "1756064805.1039"
	jobStatus, err := client.GetJobStatus(sid)

	if err != nil {
		t.Fatalf("GetJobStatus returned an error: %v", err)
	}

	earliestTime, _ := time.Parse(time.RFC3339, "2025-08-23T19:46:45.000+00:00")
	latestTime, _ := time.Parse(time.RFC3339, "2025-08-24T19:46:45.000+00:00")

	expected := SearchJobContent{
		SID:                 "1756064805.1039",
		ResultCount:         154569,
		IsDone:              true,
		IsFailed:            false,
		DispatchState:       "DONE",
		DoneProgress:        1.0,
		EarliestTime:        earliestTime,
		LatestTime:          latestTime,
		EventCount:          154569,
		EventAvailableCount: 0,
		RunDuration:         0.522,
	}

	// Make sure unmarshalling works as intended
	assertJobContentEqual(t, expected, jobStatus)
}
