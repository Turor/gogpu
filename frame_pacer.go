package gogpu

import (
	"sync/atomic"
	"time"
)

// framePacer spaces continuous frames at most maxFPS per second. Deadlines
// advance by a fixed period so the average rate holds despite sleep jitter;
// a loop that falls more than one period behind resynchronizes instead of
// bursting to catch up.
type framePacer struct {
	maxFPS atomic.Int64
	next   time.Time // render-loop goroutine only
}

func (p *framePacer) setMaxFPS(fps int) {
	if fps < 0 {
		fps = 0
	}
	p.maxFPS.Store(int64(fps))
}

// delay returns how long to wait at now before starting the next frame and
// schedules the frame after it.
func (p *framePacer) delay(now time.Time) time.Duration {
	fps := p.maxFPS.Load()
	if fps <= 0 {
		p.next = time.Time{}
		return 0
	}
	period := time.Second / time.Duration(fps)
	if p.next.IsZero() || now.Sub(p.next) > period {
		p.next = now
	}
	wait := p.next.Sub(now)
	if wait < 0 {
		wait = 0
	}
	p.next = p.next.Add(period)
	return wait
}
