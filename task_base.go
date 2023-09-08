package main

import (
	"io/fs"
	"os"
	"sync"
	"syscall"
)

type Task interface {
	Do() bool
}

type BaseTask struct {
	w    *Walker
	path string
	name string

	srcStat fs.FileInfo
	dstStat fs.FileInfo

	mu  *sync.RWMutex
	sub Task
}

func (t *BaseTask) dstName() string {
	return t.w.dst + t.path + t.name
}
func (t *BaseTask) srcName() string {
	return t.w.src + t.path + t.name
}

func (t *BaseTask) wrap(fn func() error) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	defer t.w.wg.Done()
	err := fn()
	if err != nil {
		t.w.errCh <- err
		return false
	}
	if t.sub != nil {
		t.w.wg.Add(1)
		go func() {
			t.w.taskCh <- t.sub
		}()
	}
	return true
}

func (t *BaseTask) setChown() error {
	stat := t.srcStat.Sys().(*syscall.Stat_t)
	return os.Chown(t.dstName(), int(stat.Uid), int(stat.Gid))
}

func (t *BaseTask) hasSamePermission() bool {
	if t.srcStat.Mode() != t.dstStat.Mode() {
		return false
	}

	srcStat := t.srcStat.Sys().(*syscall.Stat_t)
	dstStat := t.dstStat.Sys().(*syscall.Stat_t)

	if srcStat.Uid != dstStat.Uid {
		return false
	}
	if srcStat.Gid != dstStat.Gid {
		return false
	}

	return true
}
