//go:build linux

package wayland

import (
	"runtime"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The pipe read helpers own the read fd and must close it exactly once before
// returning. Callers no longer close it. If a helper left it open, callers
// would have to close the raw fd while an *os.File finalizer still owned the
// number, and the next GC would close whatever descriptor reused it (in GoRO,
// the char- or map-server socket), failing later writes with EBADF.
func checkPipeReaderOwnsFD(t *testing.T, read func(fd int) ([]byte, error)) {
	t.Helper()
	for i := 0; i < 20; i++ {
		var p [2]int
		if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC); err != nil {
			t.Fatal(err)
		}
		if _, err := syscall.Write(p[1], []byte("text")); err != nil {
			t.Fatal(err)
		}
		syscall.Close(p[1])
		data, err := read(p[0])
		if err != nil || string(data) != "text" {
			t.Fatalf("read = %q, %v", data, err)
		}
		if _, err := unix.FcntlInt(uintptr(p[0]), unix.F_GETFD, 0); err != unix.EBADF {
			syscall.Close(p[0])
			t.Fatalf("helper returned with fd %d still open (err=%v)", p[0], err)
		}

		// The freed number is reused by the next descriptor we open. No
		// finalizer may close it behind our back.
		var victim [2]int
		if err := syscall.Pipe2(victim[:], syscall.O_CLOEXEC); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		runtime.GC()
		time.Sleep(5 * time.Millisecond)
		_, werr := syscall.Write(victim[1], []byte("x"))
		syscall.Close(victim[0])
		syscall.Close(victim[1])
		if werr != nil {
			t.Fatalf("iteration %d: reused fd closed by a stale finalizer: %v", i, werr)
		}
	}
}

func TestReadPipeWithTimeoutDoesNotCloseReusedFD(t *testing.T) {
	checkPipeReaderOwnsFD(t, func(fd int) ([]byte, error) {
		return readPipeWithTimeout(fd, time.Second)
	})
}

func TestReadDnDPipeDoesNotCloseReusedFD(t *testing.T) {
	checkPipeReaderOwnsFD(t, func(fd int) ([]byte, error) {
		return readDnDPipe(fd, time.Second)
	})
}

func TestNewLibwaylandHandleMarksCSDFDsUnused(t *testing.T) {
	h := newLibwaylandHandle()
	for i, fd := range h.csdFDs {
		if fd != -1 {
			t.Fatalf("csdFDs[%d] = %d, want -1 (0 is stdin)", i, fd)
		}
	}
}
