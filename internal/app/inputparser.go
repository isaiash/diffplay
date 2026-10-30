package app

import (
	"strconv"
	"strings"
)

// Both requests are read-only; the existing input loop handles replies without waiting.
const terminalColorQueries = "\x1b]11;?\x1b\\\x1b]4;8;?\x1b\\"

func (u *ui) terminalColorReply(reply string) {
	var target *rgb
	switch {
	case strings.HasPrefix(reply, "11;rgb:"):
		target = &u.terminalBackground
		reply = strings.TrimPrefix(reply, "11;rgb:")
	case strings.HasPrefix(reply, "4;8;rgb:"):
		target = &u.terminalSelectedBackground
		reply = strings.TrimPrefix(reply, "4;8;rgb:")
	default:
		return
	}
	channels := strings.Split(reply, "/")
	if len(channels) != 3 {
		return
	}
	var values [3]int
	for i, channel := range channels {
		if len(channel) < 1 || len(channel) > 4 {
			return
		}
		value, err := strconv.ParseUint(channel, 16, 16)
		if err != nil {
			return
		}
		maximum := uint64(1)<<(4*len(channel)) - 1
		values[i] = int((value*255 + maximum/2) / maximum)
	}
	*target = rgb{r: values[0], g: values[1], b: values[2], explicit: values == [3]int{}}
}
