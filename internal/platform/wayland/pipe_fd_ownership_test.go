//go:build linux

package wayland

import (
	"runtime"
	"syscall"
	"testing"
	"time"
)

// The pipe read helpers own the read fd and close it exactly once. They must
// not leave an *os.File finalizer behind for an fd that was already closed. When it does, the next GC closes
// whatever unrelated descriptor reused that number, such as a game's TCP
// socket, and later writes fail with EBADF.
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

		// The freed number is reused by the next descriptor we open.
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
			t.Fatalf("iteration %d: unrelated fd closed behind our back after pipe read: %v", i, werr)
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
