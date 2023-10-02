package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
	"github.com/spf13/pflag"
)

func main() {
	var jobCount int
	pflag.IntVarP(&jobCount, "jobs", "j", maxParallelism(), "max jobs")

	var dryRun bool
	pflag.BoolVarP(&dryRun, "dry-run", "n", false, "Run diff without copy")

	var verbose bool
	pflag.BoolVarP(&verbose, "verbose", "v", false, "Show debug log")

	// parse flags
	pflag.Parse()

	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, &tint.Options{TimeFormat: "15:04:05.000", Level: logLevel})))

	if pflag.NArg() != 2 {
		fmt.Printf("Usage: %s [options] src_dir dst_dir\nOptions:\n", os.Args[0])
		pflag.PrintDefaults()
		os.Exit(1)
	}

	src := pflag.Arg(0)
	dst := pflag.Arg(1)

	var err error
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(err)

	taskCh := make(chan Task, jobCount)
	defer close(taskCh)

	for i := 0; i < jobCount; i++ {
		go worker(ctx, taskCh)
	}

	err = syncDir(ctx, src, dst, taskCh, int64(jobCount), dryRun)
	if err != nil {
		slog.Error("error when syncing", "err", err)
	}
}
