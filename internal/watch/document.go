package watch

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"weak"
)

var documents = struct {
	sync.Mutex
	all map[string]weak.Pointer[Document]
}{all: make(map[string]weak.Pointer[Document])}

// CloseDocuments releases process-owned preview and capture snapshots on shutdown.
func CloseDocuments() {
	documents.Lock()
	live := make([]*Document, 0, len(documents.all))
	for _, p := range documents.all {
		if d := p.Value(); d != nil {
			live = append(live, d)
		}
	}
	documents.Unlock()
	for _, d := range live {
		d.Close()
	}
}

// Document is an immutable disk-backed text snapshot. Only the requested window
// is retained; offsets live on disk too, so a million lines do not grow the heap.
type Document struct {
	mu             sync.Mutex
	dir            string
	data, index    *os.File
	count          int
	start          int
	cache          []string
	Added, Removed int
	Binary         bool
	TextKnown      bool
	FirstEdit      int
	HasEdit        bool
}

const maxDisplayLine = 16 * 1024
const maxDocumentBytes = 128 * 1024 * 1024

func NewDocument(r io.Reader) (*Document, error) {
	dir, err := os.MkdirTemp("", "diffplay-")
	if err != nil {
		return nil, err
	}
	d := &Document{dir: dir, start: -1}
	fail := func(err error) (*Document, error) { d.Close(); return nil, err }
	d.data, err = os.OpenFile(filepath.Join(dir, "text"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	d.index, err = os.OpenFile(filepath.Join(dir, "index"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fail(err)
	}
	br := bufio.NewReaderSize(io.LimitReader(r, maxDocumentBytes+1), 32*1024)
	dataWriter, indexWriter := bufio.NewWriterSize(d.data, 64*1024), bufio.NewWriterSize(d.index, 64*1024)

	counter := &textCounter{}
	var offset int64
	var idx [16]byte
	inHunk := false
	position := 0
	for {
		binary.LittleEndian.PutUint64(idx[:], uint64(offset))
		var prefix []byte
		size := 0
		for {
			part, e := br.ReadSlice('\n')
			if len(prefix) < maxDisplayLine {
				prefix = append(prefix, part[:min(len(part), maxDisplayLine-len(prefix))]...)
			}
			if _, err = dataWriter.Write(part); err != nil {
				return fail(err)
			}
			_, _ = counter.Write(part)
			size += len(part)
			offset += int64(len(part))
			if offset > maxDocumentBytes {
				return fail(fmt.Errorf("diff exceeds %d byte disk limit", maxDocumentBytes))
			}
			if e == bufio.ErrBufferFull {
				continue
			}
			if e != nil && e != io.EOF {
				return fail(e)
			}
			break
		}
		if size == 0 {
			break
		}
		linePosition := position
		line := string(prefix)
		if strings.HasPrefix(line, "Binary files ") {
			d.Binary = true
		}
		if strings.HasPrefix(line, "@@ ") {
			var a, b, c, e int
			if _, err := fmt.Sscanf(line, "@@ -%d,%d +%d,%d @@", &a, &b, &c, &e); err == nil {
				linePosition = c
				position = c
				inHunk = true
			}
		} else if inHunk && len(line) > 0 {
			switch line[0] {
			case '+':
				d.Added++
				if !d.HasEdit {
					d.FirstEdit = position
					d.HasEdit = true
				}
				position++
			case '-':
				d.Removed++
				if !d.HasEdit {
					d.FirstEdit = position
					d.HasEdit = true
				}
			case ' ':
				position++
			}
		}
		binary.LittleEndian.PutUint64(idx[8:], uint64(linePosition))
		if _, err = indexWriter.Write(idx[:]); err != nil {
			return fail(err)
		}
		d.count++
	}
	if err = dataWriter.Flush(); err != nil {
		return fail(err)
	}
	if err = indexWriter.Flush(); err != nil {
		return fail(err)
	}
	d.TextKnown = !counter.invalid && len(counter.pending) == 0
	documents.Lock()
	documents.all[d.dir] = weak.Make(d)
	documents.Unlock()
	runtime.SetFinalizer(d, func(d *Document) { d.Close() })
	return d, nil
}
func (d *Document) Available() bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.data != nil
}

func (d *Document) Len() int {
	if d == nil {
		return 0
	}
	return d.count
}
func (d *Document) Offset(target int) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.index == nil || target <= 0 {
		return 0
	}
	lo, hi := 0, d.count
	var buf [8]byte
	for lo < hi {
		mid := lo + (hi-lo)/2
		if _, err := d.index.ReadAt(buf[:], int64(mid)*16+8); err != nil {
			return 0
		}
		if binary.LittleEndian.Uint64(buf[:]) < uint64(target) {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == d.count {
		return 0
	}
	return max(0, lo-3)
}
func (d *Document) Window(start, n int) []string {
	if d == nil || n <= 0 {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	start = max(0, min(start, d.count))
	end := min(d.count, start+n)
	if start >= d.start && end <= d.start+len(d.cache) {
		return d.cache[start-d.start : end-d.start]
	}
	if d.data == nil {
		return []string{"Diff snapshot unavailable."}
	}
	lo, hi := max(0, start-16), min(d.count, end+32)
	offsets := make([]byte, (hi-lo+1)*16)
	got, err := d.index.ReadAt(offsets[:(hi-lo)*16], int64(lo)*16)
	if err != nil && err != io.EOF || got != (hi-lo)*16 {
		return []string{"Diff index unavailable."}
	}
	stat, err := d.data.Stat()
	if err != nil {
		return []string{"Diff snapshot unavailable."}
	}
	binary.LittleEndian.PutUint64(offsets[(hi-lo)*16:], uint64(stat.Size()))
	if hi < d.count {
		if _, err = d.index.ReadAt(offsets[(hi-lo)*16:], int64(hi)*16); err != nil {
			return []string{"Diff index unavailable."}
		}
	}
	cache := make([]string, 0, hi-lo)
	for i := 0; i < hi-lo; i++ {
		a, b := binary.LittleEndian.Uint64(offsets[i*16:]), binary.LittleEndian.Uint64(offsets[(i+1)*16:])
		length := b - a
		buf := make([]byte, min(length, maxDisplayLine))
		if _, err = d.data.ReadAt(buf, int64(a)); err != nil && err != io.EOF {
			return []string{"Diff snapshot unavailable."}
		}
		text := strings.TrimSuffix(string(buf), "\n")
		if length > maxDisplayLine {
			text += " … [long line clipped]"
		}
		cache = append(cache, text)
	}
	d.start, d.cache = lo, cache
	return cache[start-lo : end-lo]
}
func (d *Document) Text(limit int) string {
	if d == nil {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.data == nil {
		return "Diff snapshot unavailable.\n"
	}
	b := make([]byte, limit)
	n, _ := d.data.ReadAt(b, 0)
	return string(b[:n])
}
func (d *Document) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.data != nil {
		d.data.Close()
		d.data = nil
	}
	if d.index != nil {
		d.index.Close()
		d.index = nil
	}
	os.RemoveAll(d.dir)
	documents.Lock()
	delete(documents.all, d.dir)
	documents.Unlock()
}
