package main

import (
	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

var inodeCount uint64

type Node interface {
	fs.Node
}

type Dir struct {
	Fs *FS
	Name string
	Attributes fuse.Attr
	Entries    map[string]*Dir
}

func NewDir(fs *FS, name string) *Dir {
	atomic.AddUint64(&inodeCount, 1)
	return &Dir{
		Fs: fs,
		Name: name,
		Attributes: fuse.Attr{
			Inode: inodeCount,
			Atime: time.Now(),
			Mtime: time.Now(),
			Ctime: time.Now(),
			Mode:  os.ModeDir | 0o755,
			Nlink: 2,
			Uid: uint32(os.Getuid()),
			Gid: uint32(os.Getgid()),
		},
		Entries: make(map[string]*Dir),
	}
}

func (d *Dir) Attr(ctx context.Context, a *fuse.Attr) error {
	*a = d.Attributes
	return nil
}

func (d *Dir) Lookup(ctx context.Context, name string) (fs.Node, error) {
	if d == d.Fs.RootDir {
		if node, ok := d.Entries[name]; ok {
			return node, nil
		} else {
			nd := NewDir(d.Fs, name)
			d.Entries[name] = nd
			return nd, nil
		}
	}
	return nil, syscall.ENOENT
}

func (d *Dir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var entries []fuse.Dirent
	for k, v := range d.Entries {
		var a fuse.Attr
		v.Attr(ctx, &a)
		entries = append(entries, fuse.Dirent{
			Inode: a.Inode,
			Type:  fuse.DT_Dir,
			Name:  k,
		})
	}
	return entries, nil
}

type FS struct {
	RootDir *Dir
}

func NewFS() *FS {
	fs := &FS{}
	fs.RootDir = NewDir(fs, "")
	return fs
}

func (fs FS) Root() (fs.Node, error) {
	return fs.RootDir, nil
}

func main() {
	flag.Usage = func () {
		fmt.Fprintf(os.Stderr, "usage: %s [ mnt ]\n", os.Args[0])
		os.Exit(1)
	}
	flag.Parse()
	if flag.NArg() >= 2 {
		flag.Usage()
	}
	mtpt := "/n"
	if flag.NArg() == 1 {
		mtpt = flag.Arg(0)
	}
	c, err := fuse.Mount(mtpt, fuse.FSName("mntgen"), fuse.Subtype("mntgen"), fuse.AllowOther())
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for {
			<-sigc
			if err := fuse.Unmount(mtpt); err != nil {
				log.Print("unmount failed: ", err)
			}
		}
	}()
	err = fs.Serve(c, NewFS())
	if err != nil {
		log.Fatal(err)
	}
}
