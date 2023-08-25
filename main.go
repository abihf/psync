package main

import (
	"fmt"
	"log/slog"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <src_dir> <dest_dir>", os.Args[0])
		os.Exit(1)
	}

	src := os.Args[1]
	dst := os.Args[2]

	maxProc := maxParallelism()

	taskCh := make(chan Task, maxParallelism())
	doneCh := make(chan bool)
	for i := 0; i < maxProc; i++ {
		go worker(doneCh, taskCh)
	}

	errCh := syncDir(src, dst, taskCh)
	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("error when syncing", "err", err)
		}
	case <-doneCh:
		break
	}
	close(taskCh)
}
