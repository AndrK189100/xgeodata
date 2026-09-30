package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func runGenerate(inputDir, outputDir string) error {
	filesData, err := readFiles(inputDir)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	if err := saveGeoData(filesData, outputDir); err != nil {
		return fmt.Errorf("save output: %w", err)
	}

	return nil
}

func watchDir(ctx context.Context, inputDir, outputDir, host string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(inputDir); err != nil {
		return fmt.Errorf("watch dir %q: %w", inputDir, err)
	}

	log.Printf("Daemon started, watching %s", inputDir)

	// Regeneration runs in the debounce timer goroutine; its error is
	// passed back here so the daemon stops through the normal return path.
	genErr := make(chan error, 1)

	var mu sync.Mutex

	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	defer func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil

		case err := <-genErr:
			return fmt.Errorf("regenerate after change: %w", err)

		case event, ok := <-watcher.Events:
			if !ok {
				return errors.New("file watcher events channel closed unexpectedly")
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				ext := filepath.Ext(event.Name)

				if ext == ".cidr" || ext == ".domain" {
					log.Printf("Change detected: %s (%s)", filepath.Base(event.Name), event.Op)

					if debounceTimer != nil {
						debounceTimer.Stop()
					}

					debounceTimer = time.AfterFunc(debounceDuration, func() {
						mu.Lock()
						defer mu.Unlock()
						if err := runGenerate(inputDir, outputDir); err != nil {
							select {
							case genErr <- err:
							default:
							}
							return
						}
						if err := ReloadRequest(host); err != nil {
							log.Printf("Geo rules reload error: %v", err)
						}
					})
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("file watcher errors channel closed unexpectedly")
			}
			return fmt.Errorf("file watcher: %w", err)
		}
	}
}

func main() {
	inputDir := flag.String("in", "./data", "Input dir, *.domain *.cidr files")
	outputDir := flag.String("out", "./output", "Output dir .dat files")
	watchMode := flag.Bool("watch", false, "Run as daemon, monitoring input dir")
	host := flag.String("api", "", "API listen address, e. g., 127.0.0.1:10085")

	flag.Parse()

	inInfo, err := os.Stat(*inputDir)
	if err != nil {
		log.Fatalf("Input dir %q is not accessible: %v", *inputDir, err)
	}
	if !inInfo.IsDir() {
		log.Fatalf("Input path %q is not a directory", *inputDir)
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Can't create output dir %q: %v", *outputDir, err)
	}

	if err := runGenerate(*inputDir, *outputDir); err != nil {
		log.Fatalf("Generation failed: %v", err)
	}

	if err := ReloadRequest(*host); err != nil {
		log.Printf("Geo rules reload error: %v", err)
	}

	if !*watchMode {
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := watchDir(ctx, *inputDir, *outputDir, *host); err != nil {
		log.Fatalf("Daemon stopped with error: %v", err)
	}

	log.Println("Daemon stopped by signal.")
}
