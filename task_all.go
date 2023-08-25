package main

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
)

// ================================================================

type MkdirTask struct {
	BaseTask

	newMu *sync.RWMutex
}

func (t *MkdirTask) Run() error {
	perm := t.srcStat.Mode().Perm()
	t.w.log.Info("mkdir", "parent", t.path, "name", t.name, "perm", perm.String())
	if t.w.dryRun {
		t.newMu.Unlock()
		return nil
	}
	err := os.Mkdir(t.dstName(), perm)
	if err != nil {
		return err
	}
	t.newMu.Unlock()
	return nil
}

// ================================================================

type CopyTask struct {
	BaseTask
}

func (t *CopyTask) Run() error {
	if t.srcStat.Mode()&fs.ModeSymlink != 0 {
		return t.copySymlink()
	}
	if !t.srcStat.Mode().IsRegular() {
		t.w.log.Warn("skip non regular file", "path", t.path, "name", t.name)
		return nil
	}

	err := t.copyFile()
	if err != nil {
		return err
	}
	return nil // os.Chmod(t.dstName(), t.src.Type().Perm())
}

func (t *CopyTask) copyFile() error {
	t.w.log.Info("copy file", "parent", t.path, "name", t.name)
	if t.w.dryRun {
		return nil
	}

	src, err := os.Open(t.srcName())
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(t.dstName())
	if err != nil {
		return err
	}
	defer dst.Close()

	buf := make([]byte, 512)
	_, err = io.CopyBuffer(dst, src, buf)
	if err != nil {
		return err
	}
	dst.Close()

	err = t.setChown()
	if err != nil {
		return err
	}

	err = os.Chmod(t.dstName(), t.srcStat.Mode().Perm())
	if err != nil {
		return err
	}

	return nil
}

func (t *CopyTask) copySymlink() error {
	t.w.log.Info("copy symlink", "parent", t.path, "name", t.name)
	if t.w.dryRun {
		return nil
	}

	link, err := os.Readlink(t.srcName())
	if err != nil {
		return fmt.Errorf("can not read link: %w", err)
	}

	return os.Symlink(link, t.dstName())
}

// ================================================================

type DeleteFileTask struct {
	BaseTask
	reason string
}

func (t *DeleteFileTask) Run() error {
	t.w.log.Info("delete file", "parent", t.path, "name", t.name, "reason", t.reason)
	if t.w.dryRun {
		return nil
	}
	return os.Remove(t.dstName())
}

// ================================================================

type DeleteDirTask struct {
	BaseTask
	reason string
}

func (t *DeleteDirTask) Run() error {
	t.w.log.Info("delete dir", "parent", t.path, "name", t.name, "reason", t.reason)
	if t.w.dryRun {
		return nil
	}
	return os.RemoveAll(t.dstName())
}

// ================================================================

type SetPermTask struct {
	BaseTask
}

func (t *SetPermTask) Run() error {
	srcPerm := t.srcStat.Mode().Perm()
	dstPerm := t.dstStat.Mode().Perm()
	t.w.log.Info("update permission", "parent", t.path, "name", t.name,
		"before", dstPerm.String(), "after", srcPerm.String())
	if t.w.dryRun {
		return nil
	}
	err := t.setChown()
	if err != nil {
		return err
	}
	return os.Chmod(t.dstName(), srcPerm)
}
