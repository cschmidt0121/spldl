package config

type DownloaderConfig struct {
	OutputMode     string // raw, ndjson, csv
	MaxConnections int    // max concurrent connections to use for downloading results
	DeleteWhenDone bool   // delete the job when done downloading
	SID            string // the SID of the job to download results from
	Filename       string // the filename to save the results to
}
