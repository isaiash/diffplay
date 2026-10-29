package watch

import "io"

// CountTextLines streams a whole file without retaining its content or hashing.
// Invalid UTF-8 and NUL bytes make its line count unknown.
func CountTextLines(r io.Reader) (int, bool, error) {
	c := &textCounter{}
	if _, err := io.Copy(c, r); err != nil {
		return 0, false, err
	}
	if c.invalid || len(c.pending) != 0 {
		return 0, false, nil
	}
	n := c.lines
	if c.bytes > 0 && c.last != '\n' {
		n++
	}
	return n, true, nil
}
