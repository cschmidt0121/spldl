package downloader

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/cschmidt0121/spldl/internal/config"
	"github.com/cschmidt0121/spldl/internal/splunkclient"
)

const chunkSize = 10000

// eventChunk represents a downloaded chunk of events
type eventChunk struct {
	offset int
	data   string
}

type Downloader struct {
	client         *splunkclient.Client
	outputMode     string
	maxConnections int
	deleteWhenDone bool
	sid            string
	filename       string
}

func NewDownloader(client *splunkclient.Client, config config.DownloaderConfig) *Downloader {
	return &Downloader{
		client:         client,
		outputMode:     config.OutputMode,
		maxConnections: config.MaxConnections,
		deleteWhenDone: config.DeleteWhenDone,
		sid:            config.SID,
		filename:       config.Filename,
	}
}

func (d *Downloader) DownloadSearchResults() error {
	slog.Debug("Starting download process", "sid", d.sid, "output_mode", d.outputMode, "max_connections", d.maxConnections)

	// Get job status to determine total result count
	jobStatus, err := d.client.GetJobStatus(d.sid)
	if err != nil {
		return fmt.Errorf("failed to get job status: %w", err)
	}

	slog.Info("Job status retrieved", "sid", d.sid, "result_count", jobStatus.ResultCount, "dispatch_state", jobStatus.DispatchState, "is_done", jobStatus.IsDone, "is_failed", jobStatus.IsFailed)

	if !jobStatus.IsDone {
		return fmt.Errorf("job %s is not complete (state: %s, progress: %.1f%%)",
			d.sid, jobStatus.DispatchState, jobStatus.DoneProgress*100)
	}

	if jobStatus.IsFailed {
		return fmt.Errorf("job %s has failed", d.sid)
	}

	if jobStatus.ResultCount > 500000 {
		return fmt.Errorf("job %s has more than 500000 results. Split your search into multiple jobs.", d.sid)
	}

	totalChunks := (jobStatus.ResultCount / 10000) + 1
	slog.Info("Starting download", "total_chunks", totalChunks, "chunk_size", chunkSize, "max_connections", d.maxConnections)

	err = d.downloadJobChunks(totalChunks)
	if err != nil {
		return fmt.Errorf("failed to download job: %w", err)
	}

	if d.deleteWhenDone {
		slog.Debug("Deleting search job", "sid", d.sid)
		err = d.client.DeleteSearchJob(d.sid)
		if err != nil {
			return fmt.Errorf("failed to delete job: %w", err)
		}
		slog.Debug("Search job deleted successfully", "sid", d.sid)
	}

	slog.Info("Download completed successfully", "sid", d.sid, "filename", d.filename)
	return nil
}

func (d *Downloader) downloadJobChunks(totalChunks int) error {
	slog.Debug("Initializing chunk download", "total_chunks", totalChunks)
	offsetChan := make(chan int, 100)
	chunkChan := make(chan eventChunk, 100)

	// Start chunk workers
	var workerWg sync.WaitGroup
	slog.Debug("Starting worker goroutines", "worker_count", d.maxConnections)
	for range d.maxConnections {
		workerWg.Go(func() { d.chunkWorker(chunkChan, offsetChan) })
	}

	// Start collector
	var collectorWg sync.WaitGroup
	slog.Debug("Starting collector goroutine")
	collectorWg.Go(func() { d.eventChunkCollector(chunkChan) })

	// Send offsets to workers
	slog.Debug("Dispatching chunk offsets to workers")
	for i := 0; i < totalChunks; i++ {
		offsetChan <- i
	}
	close(offsetChan)
	slog.Debug("All chunk offsets dispatched")

	// Wait for workers to complete
	slog.Debug("Waiting for workers to complete")
	workerWg.Wait()
	close(chunkChan)
	slog.Debug("All workers completed")

	// Wait for collector to finish
	slog.Debug("Waiting for collector to finish")
	collectorWg.Wait()
	slog.Debug("Collector finished")

	return nil
}

func (d *Downloader) chunkWorker(chunkChan chan eventChunk, offsetChan chan int) {
	for offset := range offsetChan {
		d.getEventChunk(chunkChan, offset)
	}
}

func (d *Downloader) getEventChunk(chunkChan chan eventChunk, offset int) {
	response, err := d.client.GetJobResults(d.sid, chunkSize, offset, d.outputMode)
	if err != nil {
		slog.Error("Error getting event chunk", "error", err, "offset", offset)
		return
	}

	chunkChan <- eventChunk{
		offset: offset,
		data:   response,
	}
}

func (d *Downloader) eventChunkCollector(chunkChannel chan eventChunk) {
	slog.Debug("Starting chunk collector", "filename", d.filename)
	chunkBuf := make(map[int]eventChunk)

	outputFile, err := os.Create(d.filename)
	if err != nil {
		slog.Error("Error creating output file", "error", err, "filename", d.filename)
		return
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	defer writer.Flush()

	nextOffset := 0
	chunksWritten := 0

	for chunk := range chunkChannel {
		slog.Debug("Received chunk", "offset", chunk.offset, "expected_offset", nextOffset, "buffered_chunks", len(chunkBuf))

		if chunk.offset == nextOffset {
			// Write the chunk we need next
			writer.WriteString(chunk.data)
			nextOffset++
			chunksWritten++
			slog.Debug("Wrote chunk in order", "offset", chunk.offset, "chunks_written", chunksWritten)
		} else {
			// Buffer chunks that arrive out of order
			chunkBuf[chunk.offset] = chunk
			slog.Debug("Buffered out-of-order chunk", "offset", chunk.offset, "expected_offset", nextOffset)
		}

		// Write any buffered chunks that are now in order
		for bufferedChunk, exists := chunkBuf[nextOffset]; exists; bufferedChunk, exists = chunkBuf[nextOffset] {
			delete(chunkBuf, nextOffset)
			writer.WriteString(bufferedChunk.data)
			nextOffset++
			chunksWritten++
			slog.Debug("Wrote buffered chunk", "offset", bufferedChunk.offset, "chunks_written", chunksWritten)
		}
	}

	slog.Debug("Chunk collector completed", "total_chunks_written", chunksWritten, "filename", d.filename)
}
