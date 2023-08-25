package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"
)

type Walker struct {
	src, dst string
	dryRun   bool
	log      *slog.Logger
	wg       sync.WaitGroup
	taskCh   chan Task
	errCh    chan error
}

type SyncDirTask struct {
	w    *Walker
	path string
	mu   *sync.RWMutex
}

// Done implements Task.
func (t *SyncDirTask) Done(err error) {
	t.w.wg.Done()
	if err != nil {
		t.w.errCh <- err
	}
}

// Run implements Task.
func (t *SyncDirTask) Run() error {
	path := t.path
	w := t.w

	srcDir, err := os.ReadDir(t.w.src + path)
	if err != nil {
		return fmt.Errorf("can not read source dir: %w", err)
	}

	dstDir, err := os.ReadDir(t.w.src + path)
	if err != nil {
		t.w.log.Debug("can not read dest dir", "err", err)
		dstDir = make([]fs.DirEntry, 0)
	}

	dstMap := sliceToMap(dstDir, func(e fs.DirEntry) string { return e.Name() })
	for _, srcEntry := range srcDir {
		name := srcEntry.Name()

		dstEntry, dstExist := dstMap[name]
		if dstExist {
			delete(dstMap, name)
		}

		w.add(&ProcessDirTask{t, name, srcEntry, dstEntry})
	}

	for name, dstEntry := range dstMap {
		baseTask := BaseTask{
			w:    w,
			mu:   t.mu,
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

func (*SyncDirTask) Sub() Task {
	return nil
}

func (*SyncDirTask) Wait() func() {
	return func() {}
}

var _ Task = &SyncDirTask{}

type ProcessDirTask struct {
	*SyncDirTask
	name     string
	srcEntry fs.DirEntry
	dstEntry fs.DirEntry
}

func (t *ProcessDirTask) Run() error {
	w := t.w
	path := t.path
	name := t.name
	dstEntry := t.dstEntry
	srcEntry := t.srcEntry
	mu := t.mu

	var err error

	dstExist := dstEntry != nil

	var srcStat, dstStat fs.FileInfo
	srcName := w.src + path + name
	dstName := w.dst + path + name

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
		w.add(&SyncDirTask{w, path + name + "/", newMu})
		// w.wg.Add(1)
		// go w.walkE(path+name+"/", newMu)
		return nil
	}

	copyFile := &CopyTask{baseTask}
	if !dstExist {
		w.add(copyFile)
		return nil
	}
	if dstEntry.IsDir() {
		task := &DeleteDirTask{baseTask, "src is file"}
		task.sub = copyFile
		w.add(task)
		return nil
	}

	if (srcEntry.Type().Type()) != (dstEntry.Type().Type()) {
		task := &DeleteFileTask{baseTask, "src is different mode"}
		task.sub = copyFile
		w.add(task)
		return nil
	}

	if srcStat.ModTime().Sub(dstStat.ModTime()) > time.Second {
		task := &DeleteFileTask{baseTask, fmt.Sprintf("src is newer %s > %s", srcStat.ModTime().Format("RFC3339"), dstStat.ModTime().Format("RFC3339"))}
		task.sub = copyFile
		w.add(task)
	} else if !baseTask.hasSamePermission() {
		w.add(&SetPermTask{baseTask})
	}
	return nil
}

func syncDir(src, dst string, taskCh chan Task) chan error {
	log := slog.Default()

	errCh := make(chan error)
	w := Walker{
		// ctx:    ctx,
		src:    src,
		dst:    dst,
		taskCh: taskCh,
		errCh:  errCh,
		log:    log,
		dryRun: os.Getenv("DRYRUN") == "1",
	}

	w.add(&SyncDirTask{&w, "/", &sync.RWMutex{}})
	go func() {
		w.wg.Wait()
		errCh <- nil
	}()
	return errCh
}

// func (w *Walker) walkE(path string, mu *sync.RWMutex) {
// 	err := w.walk(path, mu)
// 	if err != nil {
// 		w.errCh <- err
// 	}
// }

// func (w *Walker) walk(path string, mu *sync.RWMutex) error {
// 	defer w.wg.Done()

// 	srcDir, err := os.ReadDir(w.src + path)
// 	if err != nil {
// 		return fmt.Errorf("can not read source dir: %w", err)
// 	}

// 	dstDir, err := os.ReadDir(w.src + path)
// 	if err != nil {
// 		w.log.Debug("can not read dest dir", "err", err)
// 		dstDir = make([]fs.DirEntry, 0)
// 	}

// 	dstMap := sliceToMap(dstDir, func(e fs.DirEntry) string { return e.Name() })
// 	for _, srcEntry := range srcDir {
// 		name := srcEntry.Name()

// 		dstEntry, dstExist := dstMap[name]
// 		if dstExist {
// 			delete(dstMap, name)
// 		}

// 		w.wg.Add(1)
// 		go w.process(path, name, mu, srcEntry, dstEntry)
// 	}

// 	for name, dstEntry := range dstMap {
// 		baseTask := BaseTask{
// 			w:    w,
// 			mu:   mu,
// 			path: path,
// 			name: name,
// 		}
// 		if dstEntry.IsDir() {
// 			w.add(&DeleteDirTask{baseTask, "src doesn't exist"})
// 		} else {
// 			w.add(&DeleteFileTask{baseTask, "src doesn't exist"})
// 		}
// 	}
// 	return nil
// }

// func (w *Walker) process(path, name string, mu *sync.RWMutex, srcEntry, dstEntry fs.DirEntry) {
// 	err := w.processE(path, name, mu, srcEntry, dstEntry)
// 	if err != nil {
// 		w.errCh <- err
// 	}
// }

// func (w *Walker) processE(path, name string, mu *sync.RWMutex, srcEntry, dstEntry fs.DirEntry) error {
// 	defer w.wg.Done()
// 	var err error

// 	dstExist := dstEntry != nil

// 	var srcStat, dstStat fs.FileInfo
// 	srcName := w.src + path + name
// 	dstName := w.dst + path + name

// 	srcStat, err = os.Lstat(srcName)
// 	if err != nil {
// 		return fmt.Errorf("can not stat src file: %s/%s: %w", path, name, err)
// 	}

// 	if dstExist {
// 		dstStat, err = os.Lstat(dstName)
// 		if err != nil {
// 			return fmt.Errorf("can not stat dst file: %s/%s: %w", path, name, err)
// 		}
// 	}

// 	baseTask := BaseTask{
// 		w:       w,
// 		mu:      mu,
// 		path:    path,
// 		name:    name,
// 		srcStat: srcStat,
// 		dstStat: dstStat,
// 	}

// 	if srcEntry.IsDir() {
// 		newMu := &sync.RWMutex{}
// 		newMu.Lock()
// 		mkdir := &MkdirTask{baseTask, newMu}
// 		if !dstExist {
// 			// create dir
// 			w.add(mkdir)
// 		} else if !dstEntry.IsDir() {
// 			baseTask.sub = mkdir
// 			// delete old file and create new dir
// 			w.add(&DeleteFileTask{baseTask, "source is dir"})
// 		} else {
// 			// don't use new mutex
// 			newMu = mu

// 			// check permission
// 			if !baseTask.hasSamePermission() {
// 				w.add(&SetPermTask{baseTask})
// 			}
// 		}
// 		w.wg.Add(1)
// 		go w.walkE(path+name+"/", newMu)
// 		return nil
// 	}

// 	copyFile := &CopyTask{baseTask}
// 	if !dstExist {
// 		w.add(copyFile)
// 		return nil
// 	}
// 	if dstEntry.IsDir() {
// 		task := &DeleteDirTask{baseTask, "src is file"}
// 		task.sub = copyFile
// 		w.add(task)
// 		return nil
// 	}

// 	if (srcEntry.Type().Type()) != (dstEntry.Type().Type()) {
// 		task := &DeleteFileTask{baseTask, "src is different mode"}
// 		task.sub = copyFile
// 		w.add(task)
// 		return nil
// 	}

// 	if srcStat.ModTime().Sub(dstStat.ModTime()) > time.Second {
// 		task := &DeleteFileTask{baseTask, fmt.Sprintf("src is newer %s > %s", srcStat.ModTime().Format("RFC3339"), dstStat.ModTime().Format("RFC3339"))}
// 		task.sub = copyFile
// 		w.add(task)
// 	} else if !baseTask.hasSamePermission() {
// 		w.add(&SetPermTask{baseTask})
// 	}
// 	return nil
// }

func (w *Walker) add(task Task) {
	w.wg.Add(1)
	w.taskCh <- task
}
