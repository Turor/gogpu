//go:build linux

package platform

import (
	"errors"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/gogpu/gogpu/internal/platform/wayland"
)

type keymapRecorder struct {
	xkbKeyHandler
	calls int
	err   error
}

func (k *keymapRecorder) SetKeymapFromFD(fd int, size uint32) error {
	k.calls++
	return k.err
}

// loadCompositorKeymap must close the compositor's keymap fd on every path,
// or each wl_keyboard.keymap event (seat changes, layout switches) leaks one.
func TestLoadCompositorKeymapClosesFD(t *testing.T) {
	cases := []struct {
		name   string
		k      xkbKeyHandler
		format uint32
		calls  int
	}{
		{"loaded", &keymapRecorder{}, wayland.KeyboardKeymapFormatXKBV1, 1},
		{"load error", &keymapRecorder{err: errors.New("bad keymap")}, wayland.KeyboardKeymapFormatXKBV1, 1},
		{"not xkb v1", &keymapRecorder{}, wayland.KeyboardKeymapFormatXKBV1 + 1, 0},
		{"no handler", nil, wayland.KeyboardKeymapFormatXKBV1, 0},
	}
	for _, tc := range cases {
		var p [2]int
		if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC); err != nil {
			t.Fatal(err)
		}
		_ = loadCompositorKeymap(tc.k, tc.format, p[0], 16)
		syscall.Close(p[1])
		if _, err := unix.FcntlInt(uintptr(p[0]), unix.F_GETFD, 0); err != unix.EBADF {
			syscall.Close(p[0])
			t.Errorf("%s: keymap fd %d still open (err=%v)", tc.name, p[0], err)
		}
		if r, ok := tc.k.(*keymapRecorder); ok && r.calls != tc.calls {
			t.Errorf("%s: SetKeymapFromFD calls = %d, want %d", tc.name, r.calls, tc.calls)
		}
	}
}
