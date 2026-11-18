package gogpu

import (
	"testing"
	"time"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

func TestFramePacerHoldsRateAndResyncs(t *testing.T) {
	var p framePacer
	t0 := time.Unix(100, 0)
	if d := p.delay(t0); d != 0 {
		t.Fatalf("uncapped delay = %v", d)
	}
	p.setMaxFPS(100) // 10ms period
	steps := []struct {
		at, want time.Duration
	}{
		{0, 0}, // first capped frame
		{2 * time.Millisecond, 8 * time.Millisecond},   // early: wait for the 10ms deadline
		{13 * time.Millisecond, 7 * time.Millisecond},  // cadence kept at 20ms
		{33 * time.Millisecond, 0},                     // 3ms past the 30ms deadline: no wait
		{500 * time.Millisecond, 0},                    // hitch: resync, no burst
		{501 * time.Millisecond, 9 * time.Millisecond}, // new cadence from 500ms
	}
	for i, s := range steps {
		if d := p.delay(t0.Add(s.at)); d != s.want {
			t.Fatalf("step %d at %v: delay = %v, want %v", i, s.at, d, s.want)
		}
	}
	p.setMaxFPS(0)
	if d := p.delay(t0.Add(502 * time.Millisecond)); d != 0 {
		t.Fatalf("disabled delay = %v", d)
	}
}

func TestResolvePresentModeLowLatencyPrefersMailbox(t *testing.T) {
	all := &wgpu.SurfaceCapabilities{PresentModes: []gputypes.PresentMode{
		gputypes.PresentModeFifo, gputypes.PresentModeImmediate, gputypes.PresentModeMailbox,
	}}
	fifoOnly := &wgpu.SurfaceCapabilities{PresentModes: []gputypes.PresentMode{gputypes.PresentModeFifo}}
	cases := []struct {
		caps              *wgpu.SurfaceCapabilities
		vsync, lowLatency bool
		want              gputypes.PresentMode
	}{
		{all, true, true, gputypes.PresentModeMailbox},
		{all, true, false, gputypes.PresentModeFifo},
		{all, false, true, gputypes.PresentModeImmediate},
		{fifoOnly, true, true, gputypes.PresentModeFifo},
	}
	for _, c := range cases {
		if got := resolvePresentModeFor(c.caps, c.vsync, c.lowLatency); got != c.want {
			t.Errorf("resolvePresentModeFor(vsync=%v, lowLatency=%v) = %v, want %v", c.vsync, c.lowLatency, got, c.want)
		}
	}
}
