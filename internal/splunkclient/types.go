package splunkclient

import "time"

// SearchJobContent contains the essential search job information
type SearchJobContent struct {
	SID                 string    `json:"sid"`
	ResultCount         int       `json:"resultCount"`
	IsDone              bool      `json:"isDone"`
	IsFailed            bool      `json:"isFailed"`
	DispatchState       string    `json:"dispatchState"`
	DoneProgress        float64   `json:"doneProgress"`
	EarliestTime        time.Time `json:"earliestTime"`
	LatestTime          time.Time `json:"latestTime"`
	EventCount          int       `json:"eventCount"`
	EventAvailableCount int       `json:"eventAvailableCount"`
	RunDuration         float64   `json:"runDuration"`
}

// SearchJobEntry represents a search job entry from the API response
type SearchJobEntry struct {
	Name    string           `json:"name"`
	ID      string           `json:"id"`
	Content SearchJobContent `json:"content"`
}

// SplunkSearchResponse represents the API response for job status
type SplunkSearchResponse struct {
	Entry []SearchJobEntry `json:"entry"`
}

type NewSearchJob struct {
	SID string `json:"sid"`
}

type SearchJobResults struct {
	Preview    bool          `json:"preview"`
	InitOffset int           `json:"init_offset"`
	Messages   []interface{} `json:"messages"`
	Fields     []struct {
		Name string `json:"name"`
	} `json:"fields"`
	Results []map[string]interface{} `json:"results"`
}
