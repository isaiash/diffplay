package watch

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func diffHeaderPath(path string) string {
	if strings.ContainsAny(path, "\r\n\t\\\"") {
		return strconv.Quote(path)
	}
	return path
}

func pagedDiff(path string, a, b Entry, aok, bok bool) (*Document, error) {
	dir, err := os.MkdirTemp("", "diffplay-input-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	names := []string{filepath.Join(dir, "old"), filepath.Join(dir, "new")}
	for i, e := range []Entry{a, b} {
		f, err := os.OpenFile(names[i], os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		if e.Source != nil {
			e.Source.mu.Lock()
			if e.Source.data == nil {
				err = fmt.Errorf("captured source unavailable")
			} else {
				_, err = io.Copy(f, io.NewSectionReader(e.Source.data, 0, maxDocumentBytes))
			}
			e.Source.mu.Unlock()
		} else {
			_, err = f.Write(e.Content)
		}
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, "git", "--no-pager", "-c", "core.attributesFile=/dev/null", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--no-renames", "--diff-algorithm=myers", "--unified=3", "--", names[0], names[1])
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C")
	pipe, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = c.Start(); err != nil {
		return nil, err
	}
	// Normalize Git's optional single-line counts while streaming, preserving source lines.
	reader, writer := io.Pipe()
	go func() {
		defer pipe.Close()
		br := bufio.NewReader(pipe)
		oldName, newName := "a/"+diffHeaderPath(path), "b/"+diffHeaderPath(path)
		if !aok {
			oldName = "/dev/null"
		}
		if !bok {
			newName = "/dev/null"
		}
		_, err := fmt.Fprintf(writer, "--- %s\n+++ %s\n", oldName, newName)
		started := false
		for err == nil {
			line, e := br.ReadString('\n')
			if strings.HasPrefix(line, "@@ ") {
				started = true
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					for _, i := range []int{1, 2} {
						if !strings.Contains(fields[i], ",") {
							fields[i] += ",1"
						}
					}
					line = strings.Join(fields, " ") + "\n"
				}
			}
			if started || strings.HasPrefix(line, "Binary files ") {
				_, err = io.WriteString(writer, line)
			}
			if e != nil {
				if e != io.EOF {
					err = e
				}
				break
			}
		}
		writer.CloseWithError(err)
	}()
	d, readErr := NewDocument(reader)
	reader.Close()
	if readErr != nil {
		_ = c.Process.Kill()
	}
	waitErr := c.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if waitErr != nil {
		if e, ok := waitErr.(*exec.ExitError); !ok || e.ExitCode() != 1 {
			d.Close()
			return nil, waitErr
		}
	}
	if d.Len() == 2 {
		d.Close()
		return NewDocument(bytes.NewBufferString("--- a/" + path + "\n+++ b/" + path + "\nMetadata or uncaptured content changed\n"))
	}
	return d, nil
}
