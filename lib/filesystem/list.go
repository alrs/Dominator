package filesystem

import (
	"fmt"
	"io"
	"path"
	"syscall"
	"time"
)

var timeFormat string = "02 Jan 2006 15:04:05 MST"

func (fs *FileSystem) list(w io.Writer) error {
	return fs.DirectoryInode.list(w, "/")
}

func (inode *DirectoryInode) list(w io.Writer, name string) error {
	_, err := fmt.Fprintf(w, "%v %5d %5d %35s %s\n",
		inode.Mode, inode.Uid, inode.Gid, "", name)
	if err != nil {
		return err
	}
	for _, dirent := range inode.EntryList {
		err = dirent.inode.List(w, path.Join(name, dirent.Name))
		if err != nil {
			return err
		}
	}
	return nil
}

func (inode *RegularInode) list(w io.Writer, name string) error {
	var err error
	t := time.Unix(inode.MtimeSeconds, int64(inode.MtimeNanoSeconds))
	if inode.Size > 0 {
		_, err = fmt.Fprintf(w, "%v %5d %5d %10d %s %s %x\n",
			inode.Mode, inode.Uid, inode.Gid, inode.Size, t.Format(timeFormat),
			name, inode.Hash)
	} else {
		_, err = fmt.Fprintf(w, "%v %5d %5d %10d %s %s\n",
			inode.Mode, inode.Uid, inode.Gid, inode.Size, t.Format(timeFormat),
			name)
	}
	if err != nil {
		return err
	}
	return nil
}

func (inode *SymlinkInode) list(w io.Writer, name string) error {
	_, err := fmt.Fprintf(w, "lrwxrwxrwx %5d %5d %35s %s -> %s\n",
		inode.Uid, inode.Gid, "", name, inode.Symlink)
	if err != nil {
		return err
	}
	return nil
}

func (inode *Inode) list(w io.Writer, name string) error {
	var data string
	data = ""
	t := time.Unix(inode.MtimeSeconds, int64(inode.MtimeNanoSeconds))
	if inode.Mode&syscall.S_IFMT == syscall.S_IFBLK ||
		inode.Mode&syscall.S_IFMT == syscall.S_IFCHR {
		data = fmt.Sprintf("%#x", inode.Rdev)
	}
	_, err := fmt.Fprintf(w, "%v %5d %5d %10s %s %s\n",
		inode.Mode, inode.Uid, inode.Gid, data, t.Format(timeFormat), name)
	if err != nil {
		return err
	}
	return nil
}

func (mode FileMode) string() string {
	var buf [10]byte
	w := 1
	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if mode&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	switch {
	case mode&syscall.S_IFMT == syscall.S_IFSOCK:
		buf[0] = 's'
	case mode&syscall.S_IFMT == syscall.S_IFLNK:
		buf[0] = 'l'
	case mode&syscall.S_IFMT == syscall.S_IFREG:
		buf[0] = '-'
	case mode&syscall.S_IFMT == syscall.S_IFBLK:
		buf[0] = 'b'
	case mode&syscall.S_IFMT == syscall.S_IFDIR:
		buf[0] = 'd'
	case mode&syscall.S_IFMT == syscall.S_IFCHR:
		buf[0] = 'c'
	case mode&syscall.S_IFMT == syscall.S_IFIFO:
		buf[0] = 'p'
	default:
		buf[0] = '?'
	}
	if mode&syscall.S_ISUID != 0 {
		if mode&syscall.S_IXUSR == 0 {
			buf[3] = 'S'
		} else {
			buf[3] = 's'
		}
	}
	if mode&syscall.S_ISGID != 0 {
		if mode&syscall.S_IXGRP == 0 {
			buf[6] = 'S'
		} else {
			buf[6] = 's'
		}
	}
	if mode&syscall.S_ISVTX != 0 {
		if mode&syscall.S_IXOTH == 0 {
			buf[9] = 'T'
		} else {
			buf[9] = 't'
		}
	}
	return string(buf[:])
}
