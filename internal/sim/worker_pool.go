package sim

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// workers is the sim's persistent worker pool: one goroutine per core
// beyond the caller's, started on first use and kept for the life of the
// program. Starting goroutines for every 1/30 s step cost more than the
// work they did (on macOS, waking a parked thread takes tens of
// microseconds), so the workers stay up: between batches they spin for a
// moment and then sleep, so a paused or 1× game doesn't burn cores. The
// caller takes chunks too, so a batch never waits on a worker that
// hasn't woken.
var workers = &workerPool{}

// workerSpins is how many polls a worker spins for the next batch before
// sleeping (a few tens of microseconds). Spinning longer, to bridge the
// half millisecond between steps at fast-forward, measured slower: busy
// workers crowd the main goroutine, which still does most of a step.
const workerSpins = 4096

type workerPool struct {
	runMu sync.Mutex // one batch at a time (several simulations may share the pool)

	startOnce sync.Once
	n         int // workers besides the caller

	mu     sync.Mutex
	cond   *sync.Cond
	asleep atomic.Int32          // workers waiting on cond
	gen    atomic.Uint64         // bumped for every batch
	cur    atomic.Pointer[batch] // the latest batch
}

// batch is one run's work: chunks indices handed out one at a time. Each
// batch has its own counters, so a worker still finishing the last batch
// can't take part of the next by mistake.
type batch struct {
	fn     func(chunk int)
	chunks int64
	next   atomic.Int64
	left   atomic.Int64
}

// size is how many goroutines a batch can use: the workers and the
// caller.
func (p *workerPool) size() int {
	p.start()
	return p.n + 1
}

func (p *workerPool) start() {
	p.startOnce.Do(func() {
		p.n = max(runtime.NumCPU()-1, 0)
		p.cond = sync.NewCond(&p.mu)
		for range p.n {
			go p.loop()
		}
	})
}

// run calls fn for every chunk in [0, chunks), spread over the workers
// and the caller, and returns when all are done.
func (p *workerPool) run(chunks int, fn func(chunk int)) {
	if chunks <= 0 {
		return
	}
	p.start()
	if chunks == 1 || p.n == 0 {
		for c := range chunks {
			fn(c)
		}
		return
	}
	p.runMu.Lock()
	defer p.runMu.Unlock()
	b := &batch{fn: fn, chunks: int64(chunks)}
	b.left.Store(int64(chunks))
	p.cur.Store(b)
	if p.asleep.Load() > 0 {
		p.mu.Lock()
		p.gen.Add(1)
		p.cond.Broadcast()
		p.mu.Unlock()
	} else {
		p.gen.Add(1) // everyone's spinning: no wake-up needed
	}
	b.work()
	for spins := 0; b.left.Load() > 0; spins++ {
		if spins%64 == 63 {
			runtime.Gosched()
		}
	}
}

// work takes chunks of b until there are none left.
func (b *batch) work() {
	for {
		c := b.next.Add(1) - 1
		if c >= b.chunks {
			return
		}
		b.fn(int(c))
		b.left.Add(-1)
	}
}

// loop is a worker: wait for a new batch (spinning, then sleeping), help
// with it, repeat.
func (p *workerPool) loop() {
	var seen uint64
	for {
		g := p.gen.Load()
		for spins := 0; g == seen && spins < workerSpins; spins++ {
			if spins%256 == 255 {
				runtime.Gosched()
			}
			g = p.gen.Load()
		}
		if g == seen {
			p.mu.Lock()
			p.asleep.Add(1)
			for p.gen.Load() == seen {
				p.cond.Wait()
			}
			p.asleep.Add(-1)
			p.mu.Unlock()
			g = p.gen.Load()
		}
		seen = g
		if b := p.cur.Load(); b != nil {
			b.work()
		}
	}
}
