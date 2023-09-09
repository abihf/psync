package main

import (
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"sync"
)

// ================================================================

type MkdirTask struct {
	BaseTask

	newMu *sync.RWMutex
}

func (t *MkdirTask) Do() bool {
	return t.wrap(func() error {
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
		err = t.setChown()
		if err != nil {
			return err
		}
		t.newMu.Unlock()
		return nil
	})
}

// ================================================================

type CopyTask struct {
	BaseTask
}

func (t *CopyTask) Do() bool {
	return t.wrap(func() error {
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
		return nil
	})
}

func (t *CopyTask) copyFile() error {
	t.w.log.Info("copy file", "parent", t.path, "name", t.name)
	if t.w.dryRun {
		return nil
	}

	var tmpFile string
	for i := 0; ; i++ {
		tmpFile = fmt.Sprintf("%s%s.psync-%s-%d", t.w.dst, t.path, t.name, rand.Int())
		if !fileExist(tmpFile) {
			break
		} else if i >= 100 {
			return fmt.Errorf("can not create temporary file for copy")
		}
	}

	err := t.copyContent(tmpFile)
	if err != nil {
		os.Remove(tmpFile)
		return err
	}

	err = os.Rename(tmpFile, t.dstName())
	if err != nil {
		return err
	}

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

func (t *CopyTask) copyContent(tmpFile string) error {
	src, err := os.Open(t.srcName())
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
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

func (t *DeleteFileTask) Do() bool {
	return t.wrap(func() error {
		t.w.log.Info("delete file", "parent", t.path, "name", t.name, "reason", t.reason)
		if t.w.dryRun {
			return nil
		}
		return os.Remove(t.dstName())
	})
}

// ================================================================

type DeleteDirTask struct {
	BaseTask
	reason string
}

func (t *DeleteDirTask) Do() bool {
	return t.wrap(func() error {
		t.w.log.Info("delete dir", "parent", t.path, "name", t.name, "reason", t.reason)
		if t.w.dryRun {
			return nil
		}
		return os.RemoveAll(t.dstName())
	})
}

// ================================================================

type SetPermTask struct {
	BaseTask
}

func (t *SetPermTask) Do() bool {
	return t.wrap(func() error {
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
	})
}
