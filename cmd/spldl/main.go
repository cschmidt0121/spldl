package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/cschmidt0121/spldl/internal/config"
	"github.com/cschmidt0121/spldl/internal/downloader"
	"github.com/cschmidt0121/spldl/internal/splunkclient"
)

func main() {
	search := flag.String("search", "", "The search query to run")
	sid := flag.String("sid", "", "An already-completed search ID to download from.")
	earliest := flag.String("earliest", "-24h", "The earliest time to search from")
	latest := flag.String("latest", "now", "The latest time to search to")
	token := flag.String("token", "", "The Splunk token to use")
	username := flag.String("username", "", "The Splunk username to use")
	password := flag.String("password", "", "The Splunk password to use")
	host := flag.String("host", "", "The Splunk host to use")
	port := flag.Int("port", 8089, "The Splunk port to use")
	insecure := flag.BoolP("insecure", "k", false, "Set this to ignore TLS verification")
	deleteWhenDone := flag.BoolP("delete-when-done", "d", false, "Set this to delete the job when done downloading. Off by default")
	concurrency := flag.Int("max-connections", 8, "The maximum number of concurrent connections to use for downloading results")
	verbose := flag.BoolP("verbose", "v", false, "Enable verbose logging")
	help := flag.BoolP("help", "h", false, "Show help")
	flag.Parse()

	// Configure slog based on verbose flag
	if *verbose {
		// Verbose mode: enable debug logging while keeping default format
		slog.SetLogLoggerLevel(slog.LevelDebug)
	} else {
		// Normal mode: info level and above
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}

	args := flag.Args()

	if len(args) == 0 {
		fmt.Println("No output file specified")
		fmt.Println("Usage: spldl [options] <output-file.[json|csv|txt]>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if *help {
		fmt.Println("Usage: spldl [options] <output-file.[json|csv|txt]>")
		flag.PrintDefaults()
		os.Exit(0)
	}

	// Load environment variables
	if *token == "" {
		*token = os.Getenv("SPLUNK_TOKEN")
	}

	if *username == "" {
		*username = os.Getenv("SPLUNK_USERNAME")
	}

	if *password == "" {
		*password = os.Getenv("SPLUNK_PASSWORD")
	}

	// Validate required flags
	if *search == "" && *sid == "" {
		fmt.Println("You must provide either a search query or a search ID. Use spldl --help for more information.")
		os.Exit(1)
	}
	var auth config.AuthConfig
	if *token != "" {
		auth = config.AuthConfig{
			Type:  config.AuthToken,
			Token: *token,
		}
	} else if *username != "" && *password != "" {
		auth = config.AuthConfig{
			Type:     config.AuthHTTPBasic,
			Username: *username,
			Password: *password,
		}
	} else {
		fmt.Println("No authentication method provided. Use spldl --help for more information.")
		os.Exit(1)
	}

	filename := args[0]
	var outputMode string
	switch ext := filepath.Ext(filename); ext {
	case ".ndjson":
		outputMode = "ndjson"
	case ".csv":
		outputMode = "csv"
	case ".txt":
		outputMode = "raw"
	default:
		fmt.Println("Output file must have .json, .csv, or .txt extension")
		os.Exit(1)
	}
	clientConfig := config.ClientConfig{
		Host:      *host,
		Port:      *port,
		Auth:      auth,
		UseTLS:    true,
		VerifyTLS: !*insecure,
	}
	client := splunkclient.NewClient(clientConfig)

	if *sid == "" {
		var err error
		*sid, err = client.NewSearchJob(*search, *earliest, *latest)
		if err != nil {
			slog.Error("Failed to create search job", "error", err)
			os.Exit(1)
		}
		slog.Info("Created search job", "sid", *sid)
		slog.Info("Waiting for job to be done")
		err = client.WaitUntilJobIsDone(*sid)
		if err != nil {
			slog.Error("Failed while waiting for job to be done", "error", err)
			os.Exit(1)
		}
	}

	slog.Info("Downloading search results", "sid", *sid)

	downloaderConfig := config.DownloaderConfig{
		OutputMode:     outputMode,
		DeleteWhenDone: *deleteWhenDone,
		MaxConnections: *concurrency,
		SID:            *sid,
		Filename:       filename,
	}
	downloader := downloader.NewDownloader(client, downloaderConfig)

	err := downloader.DownloadSearchResults()
	if err != nil {
		slog.Error("Failed to download search results", "error", err)
		os.Exit(1)
	}

	slog.Info("Downloaded search results", "filename", filename)

}
