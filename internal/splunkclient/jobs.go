package splunkclient

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

func parseCSVResponse(response string, offset int) string {
	if offset > 0 {
		// Remove the first line (header) from the response if it's not the first chunk
		if idx := strings.IndexByte(response, '\n'); idx != -1 {
			response = response[idx+1:]
		}
	}
	return response
}

func parseJSONResponse(response string) string {
	var unmarshalled SearchJobResults

	err := json.Unmarshal([]byte(response), &unmarshalled)
	if err != nil {
		slog.Error("Error unmarshalling JSON", "error", err)
		return ""
	}

	var sb strings.Builder
	for _, result := range unmarshalled.Results {
		line, err := json.Marshal(result)
		if err != nil {
			slog.Error("Error marshalling result to JSON", "error", err)
			continue
		}
		sb.Write(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}
func parseResultsResponse(response string, outputMode string, offset int) string {
	switch outputMode {
	case "raw":
		return response
	case "csv":
		return parseCSVResponse(response, offset)
	case "ndjson":
		return parseJSONResponse(response)
	default:
		return ""
	}
}

func (c *Client) GetJobResults(sid string, count, offset int, outputMode string) (string, error) {
	path := fmt.Sprintf("/services/search/v2/jobs/%s/results", sid)

	queryParams := map[string]string{
		"count":       fmt.Sprintf("%d", count),
		"offset":      fmt.Sprintf("%d", offset*count),
		"output_mode": outputMode,
	}

	response, err := c.Get(path, queryParams)
	if err != nil {
		return "", err
	}

	parsed := parseResultsResponse(response, outputMode, offset)
	slog.Debug("Job results chunk processed", "sid", sid, "chunk_offset", offset, "response_size", len(response), "parsed_size", len(parsed))

	return parsed, nil
}

// GetJobStatus retrieves the status of a search job
func (c *Client) GetJobStatus(sid string) (SearchJobContent, error) {
	path := fmt.Sprintf("/services/search/v2/jobs/%s", sid)

	queryParams := map[string]string{
		"output_mode": "json",
	}

	response, err := c.Get(path, queryParams)
	if err != nil {
		return SearchJobContent{}, err
	}

	var job SplunkSearchResponse
	err = json.Unmarshal([]byte(response), &job)
	if err != nil {
		return SearchJobContent{}, fmt.Errorf("error unmarshalling job status: %w", err)
	}

	if len(job.Entry) == 0 {
		return SearchJobContent{}, fmt.Errorf("no job found for sid %s", sid)
	}

	return job.Entry[0].Content, nil
}

func (c *Client) NewSearchJob(search string, earliest string, latest string) (string, error) {
	// Check if search matches the regex pattern \s*(\||search ).*
	// If not, prepend "search " to the search string
	pattern := regexp.MustCompile(`^\s*(\||search ).*`)
	if !pattern.MatchString(search) {
		search = "search " + search
		slog.Debug("Prepended 'search ' to search string", "modified_search", search)
	}

	slog.Debug("Creating new search job", "search", search, "earliest", earliest, "latest", latest)

	path := "/services/search/jobs"
	queryParams := map[string]string{
		"output_mode": "json",
	}

	data := url.Values{
		"search":        {search},
		"earliest_time": {earliest},
		"latest_time":   {latest},
		"rf":            {"*"},
		"timeout":       {"3600"},
	}

	response, err := c.Post(path, "application/x-www-form-urlencoded", queryParams, []byte(data.Encode()))
	if err != nil {
		return "", err
	}

	var job NewSearchJob
	err = json.Unmarshal([]byte(response), &job)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling job status: %w", err)
	}

	slog.Debug("Search job created successfully", "sid", job.SID)
	return job.SID, nil
}

func (c *Client) WaitUntilJobIsDone(sid string) error {
	slog.Debug("Waiting for job to complete", "sid", sid)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		status, err := c.GetJobStatus(sid)
		if err != nil {
			return fmt.Errorf("failed to get job status: %w", err)
		}

		slog.Debug("Job status check", "sid", sid, "is_done", status.IsDone, "dispatch_state", status.DispatchState, "done_progress", status.DoneProgress)

		if status.IsDone {
			slog.Debug("Job completed successfully", "sid", sid)
			return nil
		}
	}
	return nil
}

func (c *Client) DeleteSearchJob(sid string) error {
	path := fmt.Sprintf("/services/search/v2/jobs/%s", sid)

	queryParams := map[string]string{
		"output_mode": "json",
	}

	_, err := c.Delete(path, queryParams)
	if err != nil {
		return err
	}

	return nil
}
