package hh

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// WalkText calls fn with the path and contents of every hand history text file
// (.txt) under the roots. A root is a folder, searched recursively, a .txt file,
// or a .zip archive. Archives are read where they are, without being unpacked, and
// so are the archives inside them, to any depth. A file inside an archive has
// the archive's path followed by its path in the archive, as in
// DIR\all.zip\day.zip\table.txt.
//
// fn is called from workers goroutines at once, the largest files first. A file
// that cannot be read is skipped, and the errors are returned together at the end.
func WalkText(roots []string, workers int, fn func(path string, data []byte)) error {
	type unit struct {
		path  string
		size  int64
		read  func() ([]byte, error)
		isZip bool // the data is an archive inside an archive, whose files are read in turn
	}
	var units []unit
	var errs []error
	var archives []*zip.ReadCloser
	defer func() {
		for _, a := range archives {
			a.Close()
		}
	}()
	addFile := func(p string, size int64) {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".txt":
			units = append(units, unit{path: p, size: size, read: func() ([]byte, error) { return os.ReadFile(p) }})
		case ".zip":
			a, err := zip.OpenReader(p)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", p, err))
				return
			}
			archives = append(archives, a)
			for _, f := range a.File {
				ext := strings.ToLower(filepath.Ext(f.Name))
				if ext != ".txt" && ext != ".zip" {
					continue
				}
				units = append(units, unit{path: filepath.Join(p, f.Name), size: int64(f.UncompressedSize64), read: zipEntry(f), isZip: ext == ".zip"})
			}
		}
	}
	for _, root := range roots {
		st, err := os.Stat(root)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !st.IsDir() {
			addFile(root, st.Size())
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			if !d.IsDir() {
				if info, err := d.Info(); err == nil {
					addFile(p, info.Size())
				}
			}
			return nil
		})
	}
	sort.SliceStable(units, func(i, j int) bool { return units[i].size > units[j].size })

	var mu sync.Mutex
	fail := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}
	work := make(chan unit)
	var wg sync.WaitGroup
	for range max(workers, 1) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range work {
				b, err := u.read()
				if err != nil {
					fail(fmt.Errorf("%s: %w", u.path, err))
					continue
				}
				if u.isZip {
					walkZipBytes(u.path, b, fn, fail)
				} else {
					fn(u.path, b)
				}
			}
		}()
	}
	for _, u := range units {
		work <- u
	}
	close(work)
	wg.Wait()
	return errors.Join(errs...)
}

// walkZipBytes calls fn with every text file in the archive held in b, and in the
// archives inside it, in archive order.
func walkZipBytes(path string, b []byte, fn func(string, []byte), fail func(error)) {
	a, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		fail(fmt.Errorf("%s: %w", path, err))
		return
	}
	for _, f := range a.File {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".txt" && ext != ".zip" {
			continue
		}
		p := filepath.Join(path, f.Name)
		data, err := zipEntry(f)()
		if err != nil {
			fail(fmt.Errorf("%s: %w", p, err))
			continue
		}
		if ext == ".zip" {
			walkZipBytes(p, data, fn, fail)
		} else {
			fn(p, data)
		}
	}
}

func zipEntry(f *zip.File) func() ([]byte, error) {
	return func() ([]byte, error) {
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
}

// RelText is the path of a file that WalkText found under root, relative to
// root, with the archives in it taken as folders: for root DIR and the file
// DIR\all.zip\day.zip\table.txt it is all\day\table.txt. A tool that writes one
// output file per input file uses it to lay out its output.
func RelText(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		rel = filepath.Base(path)
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for i := range parts[:len(parts)-1] {
		if strings.EqualFold(filepath.Ext(parts[i]), ".zip") {
			parts[i] = parts[i][:len(parts[i])-len(".zip")]
		}
	}
	return filepath.Join(parts...)
}

// ParseText is ParseFile for text already read, such as a file that WalkText
// found.
func ParseText(data []byte, fn func(*HandHistory) error, onErr func(error)) (Stats, error) {
	return parseAll(NewReader(bytes.NewReader(data)), fn, onErr)
}
