package app

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestListBurstTerminalRepliesAreFragmentSafe(t *testing.T) {
	reply := "\x1b]11;rgb:1313/1b1b/2323\x1b\\\x1b]4;8;rgb:4747/4c4c/5151\a"
	for split := 0; split <= len(reply); split++ {
		u := newUI(newGraph(), "/repo")
		u.followPending = []string{"keep"}
		u.input([]byte(reply[:split]))
		u.input([]byte(reply[split:]))
		if u.terminalBackground != (rgb{r: 19, g: 27, b: 35}) || u.terminalSelectedBackground != (rgb{r: 71, g: 76, b: 81}) {
			t.Fatalf("split %d: %+v / %+v", split, u.terminalBackground, u.terminalSelectedBackground)
		}
		if u.quit || u.zoom != 1 || len(u.followPending) != 1 || len(u.pending) != 0 {
			t.Fatalf("reply became input at split %d: %+v", split, u)
		}
	}
	u := newUI(newGraph(), "/repo")
	for _, b := range []byte(reply) {
		u.input([]byte{b})
	}
	if u.terminalBackground != (rgb{r: 19, g: 27, b: 35}) || u.terminalSelectedBackground != (rgb{r: 71, g: 76, b: 81}) {
		t.Fatal("bytewise reply failed")
	}
	u.input([]byte("\x1b["))
	u.input([]byte("B"))
	if u.scroll != 1 {
		t.Fatal("fragmented arrow lost after reply")
	}
	u.input([]byte("\x1b[<0;4;5"))
	u.input([]byte("M"))
	if !u.dragging {
		t.Fatal("fragmented mouse lost after reply")
	}
	u.input([]byte("q"))
	if !u.quit {
		t.Fatal("quit lost after reply")
	}
}

func TestTerminalOSCTruncationPreservesQuit(t *testing.T) {
	for _, prefix := range []string{"\x1b]11;rgb:0000/0000/0000", "\x1b]4;8;rgb:0000/0000/0000", "\x1b]11;rgb:0000/0000/0000\x1b", "\x1b]" + strings.Repeat("x", 512)} {
		for _, key := range []byte{'q', 3} {
			for _, fragmented := range []bool{false, true} {
				u := newUI(newGraph(), "/repo")
				if fragmented {
					u.input([]byte(prefix))
					u.input([]byte{key})
				} else {
					u.input(append([]byte(prefix), key))
				}
				if !u.quit || u.terminalOSCActive || len(u.terminalOSC) != 0 || len(u.pending) != 0 {
					t.Errorf("prefix %q key %d fragmented %v: quit=%v active=%v retained=%d pending=%q", prefix, key, fragmented, u.quit, u.terminalOSCActive, len(u.terminalOSC), u.pending)
				}
			}
		}
	}
}

func TestTerminalOSCTruncationExpires(t *testing.T) {
	for _, payload := range []string{"11;rgb:0000/0000/0000", strings.Repeat("x", 512)} {
		u := newUI(newGraph(), "/repo")
		u.followPending = []string{"keep"}
		u.input([]byte("\x1b]" + payload))
		if !u.terminalOSCActive || u.terminalOSCStarted.IsZero() || len(u.terminalOSC) > 129 || len(u.followPending) != 1 {
			t.Errorf("drain must start its deadline and preserve autofollow: active=%v started=%v retained=%d", u.terminalOSCActive, u.terminalOSCStarted, len(u.terminalOSC))
		}
		started := u.terminalOSCStarted
		u.input([]byte("x"))
		if u.terminalOSCStarted != started || len(u.followPending) != 1 {
			t.Fatal("draining must not renew the deadline or cancel autofollow")
		}
		u.terminalOSCStarted = time.Now().Add(-2 * time.Second)
		u.input([]byte("j"))
		if u.scroll != 1 || u.terminalOSCActive || len(u.terminalOSC) != 0 || len(u.followPending) != 0 {
			t.Errorf("expired drain swallowed input: scroll=%d active=%v retained=%d follow=%v", u.scroll, u.terminalOSCActive, len(u.terminalOSC), u.followPending)
		}
		u.input([]byte("\x1b]11;rgb:0/f/8\a"))
		if u.terminalBackground != (rgb{r: 0, g: 255, b: 136}) {
			t.Fatal("new reply failed after expiry")
		}
	}
}

func TestListBurstTerminalRepliesValidateAndBoundPayload(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	for _, reply := range []string{"11;rgb:gg/00/00", "11;rgb:10000/00/00", "11;rgb:11/22", "4;7;rgb:ff/ff/ff", "11;rgb:ff/ff/ff;extra", "11;rgb:ff/ff/ff" + strings.Repeat("x", 512)} {
		u.input([]byte("\x1b]" + reply + "\a"))
		if u.terminalBackground != (rgb{}) || u.terminalSelectedBackground != (rgb{}) || u.quit {
			t.Fatalf("invalid reply accepted or became keys: %q", reply)
		}
	}
	u.input([]byte("\x1b]11;rgb:0/f/8\a"))
	if u.terminalBackground != (rgb{r: 0, g: 255, b: 136}) {
		t.Fatalf("short hex scaling: %+v", u.terminalBackground)
	}
	u.input([]byte("\x1b]11;rgb:0000/0000/0000\x1b\\"))
	if got := u.terminalBackground.sgr(true); got != "\x1b[48;2;0;0;0m" {
		t.Fatalf("black is not unspecified: %q", got)
	}
	u.followPending = []string{"keep"}
	u.input([]byte("\x1b]broken\x1b["))
	if len(u.followPending) != 1 {
		t.Fatal("incomplete CSI canceled autofollow")
	}
	u.input([]byte("B"))
	if u.scroll != 1 || len(u.followPending) != 0 {
		t.Fatal("unterminated OSC swallowed following arrow")
	}
}

func TestListBurstBlackTargetAndFallback(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.input([]byte("\x1b]4;8;rgb:0/0/0\a"))
	got := u.listBurstBackground(selectedBackground, blue, listBurstStrength(999*time.Millisecond))
	if got.sgr(true) != "\x1b[48;2;0;0;0m" {
		t.Fatalf("near-black fade lost explicit RGB: %+v", got)
	}
	c := newCanvas(1, 1)
	c.cells[0].bg = got
	if !strings.Contains(c.ansi(), "\x1b[48;2;0;0;0m") {
		t.Fatal("canvas erased true black background")
	}
	u = newUI(newGraph(), "/repo")
	for _, base := range []rgb{background, selectedBackground} {
		first := u.listBurstBackground(base, red, 1)
		middle := u.listBurstBackground(base, red, listBurstStrength(500*time.Millisecond))
		if first == middle || middle.g < 0 || middle.dim {
			t.Fatal("fallback must fade via RGB, not dim steps")
		}
		if u.listBurstBackground(base, red, 0) != base {
			t.Fatal("fallback must restore actual palette/default at endpoint")
		}
	}
}

func TestListBurstInterpolatesEvery100msAndRestoresBase(t *testing.T) {
	u := newUI(newGraph(), "/repo")
	u.terminalBackground = rgb{r: 19, g: 27, b: 35}
	u.terminalSelectedBackground = rgb{r: 71, g: 76, b: 81}
	for _, base := range []rgb{background, selectedBackground, {r: 30, g: 40, b: 50}} {
		for _, event := range []rgb{blue, green, red} {
			target := base
			if base == background {
				target = u.terminalBackground
			}
			if base == selectedBackground {
				target = u.terminalSelectedBackground
			}
			pastel := blend(event, white, .45)
			previous := pastel
			for step := 0; step < 10; step++ {
				age := time.Duration(step) * 100 * time.Millisecond
				strength := listBurstStrength(age)
				wantStrength := math.Pow(1-float64(step)/10, 2)
				if math.Abs(strength-wantStrength) > 1e-12 {
					t.Fatalf("strength at %v = %g", age, strength)
				}
				got := u.listBurstBackground(base, event, strength)
				if got.g < 0 || got.dim {
					t.Fatalf("not an RGB background at %v: %+v", age, got)
				}
				for i, pair := range [][2]int{{target.r, pastel.r}, {target.g, pastel.g}, {target.b, pastel.b}} {
					want := int(math.Round(float64(pair[0]) + float64(pair[1]-pair[0])*strength))
					if []int{got.r, got.g, got.b}[i] != want {
						t.Fatalf("at %v: %+v, channel %d want %d", age, got, i, want)
					}
				}
				if step > 0 && got == previous {
					t.Fatalf("no fade at %v: %+v", age, got)
				}
				previous = got
			}
			for _, age := range []time.Duration{-1, time.Second, 2 * time.Second} {
				if got := u.listBurstBackground(base, event, listBurstStrength(age)); got != base {
					t.Fatalf("endpoint %v = %+v, want %+v", age, got, base)
				}
			}
		}
	}
	if listBurstDuration != time.Second {
		t.Fatal("list burst duration must be exactly one second")
	}
}
