package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

type Walker struct {
	ctx      context.Context
	src, dst string
	dryRun   bool
	log      *slog.Logger
	taskCh   chan Task
	errCh    chan error

	wg  sync.WaitGroup
	sem *semaphore.Weighted
}

func syncDir(ctx context.Context, src, dst string, taskCh chan Task, maxThread int64, dryRun bool) error {
	log := slog.Default()
	slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))

	errCh := make(chan error)
	w := Walker{
		ctx:    ctx,
		src:    src,
		dst:    dst,
		taskCh: taskCh,
		errCh:  errCh,
		log:    log,
		dryRun: dryRun,
		sem:    semaphore.NewWeighted(maxThread),
	}

	w.wg.Add(1)
	w.walkE("/", &sync.RWMutex{})
	go func() {
		w.wg.Wait()
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func (w *Walker) walkE(path string, mu *sync.RWMutex) {
	defer w.wg.Done()

	err := w.sem.Acquire(w.ctx, 1)
	if err != nil {
		w.log.Error("can not acquire semaphore", "err", err)
		return
	}
	defer w.sem.Release(1)

	err = w.walk(path, mu)
	if err != nil {
		w.errCh <- err
	}
}

func (w *Walker) walk(path string, mu *sync.RWMutex) error {
	var err error

	readdir, errs := AllSettled(os.ReadDir, w.src+path, w.dst+path)
	if errs[0] != nil {
		return fmt.Errorf("can not read source dir: %w", errs[0])
	}
	srcDir := readdir[0]
	dstDir := readdir[1]
	if errs[1] != nil {
		w.log.Debug("can not read dest dir", "err", errs[1])
		dstDir = make([]fs.DirEntry, 0)
	}

	dstMap := sliceToMap(dstDir, func(e fs.DirEntry) string { return e.Name() })
	for _, srcEntry := range srcDir {
		name := srcEntry.Name()
		srcName := w.src + path + name
		dstName := w.dst + path + name

		dstEntry, dstExist := dstMap[name]
		if dstExist {
			delete(dstMap, name)
		}
		var srcStat, dstStat fs.FileInfo

		srcStat, err = os.Lstat(srcName)
		if err != nil {
			return fmt.Errorf("can not stat src file: %s/%s: %w", path, name, err)
		}
		if dstExist {
			dstStat, err = os.Lstat(dstName)
			if err != nil {
				return fmt.Errorf("can not stat dst file: %s/%s: %w", path, name, err)
			}
		}

		baseTask := BaseTask{
			w:       w,
			mu:      mu,
			path:    path,
			name:    name,
			srcStat: srcStat,
			dstStat: dstStat,
		}

		if srcEntry.IsDir() {
			newMu := &sync.RWMutex{}
			newMu.Lock()
			mkdir := &MkdirTask{baseTask, newMu}
			if !dstExist {
				// create dir
				w.add(mkdir)
			} else if !dstEntry.IsDir() {
				baseTask.sub = mkdir
				// delete old file and create new dir
				w.add(&DeleteFileTask{baseTask, "source is dir"})
			} else {
				// don't use new mutex
				newMu = mu

				// check permission
				if !baseTask.hasSamePermission() {
					w.add(&SetPermTask{baseTask})
				}
			}
			w.wg.Add(1)
			go w.walkE(path+name+"/", newMu)
			continue
		}

		copyFile := &CopyTask{baseTask}
		if !dstExist {
			w.add(copyFile)
			continue
		}
		if dstEntry.IsDir() {
			task := &DeleteDirTask{baseTask, "src is file"}
			task.sub = copyFile
			w.add(task)
			continue
		}

		if (srcEntry.Type().Type()) != (dstEntry.Type().Type()) {
			task := &DeleteFileTask{baseTask, "src is different mode"}
			task.sub = copyFile
			w.add(task)
			continue
		}

		if srcStat.ModTime().Sub(dstStat.ModTime()) > 5*time.Second {
			task := &DeleteFileTask{baseTask, "src is newer"}
			task.sub = copyFile
			w.add(task)
		} else if !baseTask.hasSamePermission() {
			w.add(&SetPermTask{baseTask})
		}
	}

	for name, dstEntry := range dstMap {
		baseTask := BaseTask{
			w:    w,
			mu:   mu,
			path: path,
			name: name,
		}
		if dstEntry.IsDir() {
			w.add(&DeleteDirTask{baseTask, "src doesn't exist"})
		} else {
			w.add(&DeleteFileTask{baseTask, "src doesn't exist"})
		}
	}
	return nil
}

func (w *Walker) add(task Task) {
	w.wg.Add(1)
	w.taskCh <- task
}
