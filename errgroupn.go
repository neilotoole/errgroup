// Package errgroupn is an extension of the sync.errgroup
// concept, and much of the code herein is descended from
// or directly copied from that sync.errgroup code which
// has this header comment:
//
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errgroupn is a drop-in alternative to errgroup but
// limited to N goroutines. In effect, errgroupn is errgroup
// but with a worker pool of N goroutines.
package errgroupn

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"go.uber.org/atomic"
)

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid and does not cancel on error. FIXME: errgroupg should emulate this behavior.
type Group struct {
	cancel    func()
	ctxDoneCh <-chan struct{}

	wg sync.WaitGroup

	errOnce sync.Once
	err     error

	// initOnce is invoked by method Go to initialize the group.
	initOnce sync.Once

	// numG is the maximum number of goroutines that can be started.
	numG int

	// qSize is the capacity of qCh, used for buffering funcs
	// passed to method Go.
	qSize int

	// qCh is the buffer used to hold funcs passed to method Go
	// before they are picked up by worker goroutines.
	qCh chan func() error

	// qMu protects qCh.
	qMu sync.Mutex

	// qClosed is true if qCh has been closed.
	qClosed  bool
	qStopped bool

	stopCh chan struct{}

	gCount atomic.Int64

	gIdCount    atomic.Int64 // FIXME: delete
	fCount      atomic.Int64 // FIXME: delete
	workedCount atomic.Int64 // FIXME: delete
}

// WithContext returns a new Group and an associated Context derived from ctx.
// It is equivalent to WithContextN(ctx, 0, 0).
func WithContext(ctx context.Context) (*Group, context.Context) {
	return WithContextN(ctx, 0, 0)
}

// WithContextN returns a new Group and an associated Context derived from ctx.
//
// The derived Context is canceled the first time a function passed to Go
// returns a non-nil error or the first time Wait returns, whichever occurs
// first.
//
// Param numG controls the number of worker goroutines. Pram qSize
// controls the size of the queue channel that holds functions passed
// to method Go: while the queue channel is full, Go blocks.
// If numG <= 0, the value of runtime.NumCPU is used (if qSize is
// also <= 0, a qSize of runtime.NumCPU*2 is used).
func WithContextN(ctx context.Context, numG, qSize int) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{ctxDoneCh: ctx.Done(), cancel: cancel, numG: numG, qSize: qSize}, ctx
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (g *Group) Wait() error {
	printlnf("\ng.Wait...\n")
	g.qMu.Lock()

	started := g.gCount.Load()

	// if g.qClosed {
	// 	// qCh is closed already, which means this is a dodgy
	// 	// invocation of Wait. Perhaps a panic is in order. But
	// 	// we emulate the behavior of errgroup here.
	//
	// 	g.qMu.Unlock()
	// 	return g.err
	// }

	// if g.qCh != nil && !g.qClosed {
	if g.qCh != nil && !g.qClosed {
		// qCh can be nil if Wait is invoked before the first
		// call to Go (in which method qCh is initialized)
		close(g.qCh)
		g.qClosed = true
		// g.gCount.Store(0) // Reset the counter
	}

	if g.stopCh != nil && !g.qStopped {
		close(g.stopCh)
		g.qStopped = true
		// g.stopCh = nil
	}

	g.wg.Wait()
	g.qCh = nil
	g.qMu.Unlock()

	if g.cancel != nil {
		g.cancel()
	}

	printlnf("\ng.Wait: exiting (started=%d; err=%v)\n", started, g.err)
	return g.err
}

// Go adds the given function to a queue of functions that are called
// by one of g's worker goroutines.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
//
// Go may panic if invoked after Wait.
func (g *Group) Go(f func() error) {
	g.qMu.Lock()
	if g.qCh == nil {
		printlnf("g.Go: initing...")

		g.qClosed, g.qStopped = false, false

		// The zero value of numG would mean no worker G created.
		// We want the "effective" zero value to be runtime.NumCPU.
		if g.numG == 0 {
			g.numG = runtime.NumCPU()
			if g.qSize == 0 {
				// Set an arbitrary "sensible" default for qSize.
				g.qSize = g.numG * 2
			}
		}

		g.qCh = make(chan func() error, g.qSize)
		g.stopCh = make(chan struct{})
		// Being that g.Go has been invoked, we'll need at
		// least one goroutine.

		printlnf("G.Go.init: add #%d", g.fCount.Inc())

		g.gIdCount.Store(0)
		g.gCount.Store(1)
		g.startG()
		g.qCh <- f
		g.qMu.Unlock()
		return
	}

	printlnf("add #%d", g.fCount.Inc())
	g.qCh <- f

	// Yield so that already-running goroutines have a chance to
	// pick up the new f from qCh before we check if we should
	// start a new goroutine.
	runtime.Gosched()

	// Check if we can or should start a new goroutine.
	g.maybeStartG()

	g.qMu.Unlock()

	return
}

func (g *Group) maybeStartG() {
	if len(g.qCh) == 0 {
		// No point starting a new goroutine if there's nothing
		// in qCh
		return
	}

	// We have at least one item in qCh. Maybe it's time to start
	// a new goroutine?

	wouldBeCount := g.gCount.Inc()
	if wouldBeCount > int64(g.numG) {
		// Starting a new goroutine would put us over the numG limit,
		// so we back out.
		g.gCount.Dec()
		return
	}

	// It's safe to start a new goroutine
	g.startG()
}

func (g *Group) startG() {
	gCount := g.gCount.Load() // FIXME: delete
	if gCount > int64(g.numG) {
		panic(fmt.Sprintf("tried to start g when gCount is %d", gCount))
	}

	g.wg.Add(1)
	go func() {
		id := g.gIdCount.Inc()

		defer printlnf("exit g%d", id)
		defer g.wg.Done()

		printlnf("start g%d", id)

		var f func() error

		for {
			select {
			// case <-g.stopCh:
			// 	printlnf("g%d: exiting due to <--g.stopCh", id)
			// 	return
			// case <-g.ctxDoneCh:
			// 	printlnf("g%d: exiting due to <--g.ctxDoneCh", id)
			// 	return
			case f = <-g.qCh:
				if f == nil {
					// qCh was closed, time to exit
					printlnf("g%d: exiting due to <--q.qCh received nil", id)

					return
				}
			}

			workedCount := g.workedCount.Inc()
			printlnf("g%d: handling #%d", id, workedCount)

			if err := f(); err != nil {
				g.errOnce.Do(func() {
					g.err = err
					if g.cancel != nil {
						g.cancel()
					}
				})
				printlnf("g%d: exiting due to error from func", id)

				return
			}
		}
	}()
}

func printlnf(format string, a ...interface{}) {
	// println(fmt.Sprintf(format, a...))
}

//
// // Go adds the given function to a queue of functions that are called
// // by one of g's worker goroutines.
// //
// // The first call to return a non-nil error cancels the group; its error will be
// // returned by Wait.
// //
// // Go may panic if invoked after Wait.
// func (g *Group) Go(f func() error) {
// 	var didInit bool
// 	g.initOnce.Do(func() {
//
// 		// The zero value of numG would mean no worker G created.
// 		// We want the "effective" zero value to be runtime.NumCPU.
// 		if g.numG == 0 {
// 			g.numG = runtime.NumCPU()
// 			if g.qSize == 0 {
// 				// Set an arbitrary "sensible" default for qSize.
// 				g.qSize = g.numG * 2
// 			}
// 		}
//
// 		g.qCh = make(chan func() error, g.qSize)
//
// 		// Being that g.Go has been invoked, we'll need at
// 		// least one goroutine.
//
// 		fCount := g.fCount.Inc()
// 		printlnf("add #%d", fCount)
//
// 		g.gCount.Store(1)
// 		g.startG()
// 		didInit = true
// 	})
//
// 	if didInit {
// 		return
// 	}
//
// 	fCount := g.fCount.Inc()
// 	printlnf("add #%d", fCount)
//
// 	g.qMu.Lock()
// 	if g.qClosed {
// 		// g.qMu.Unlock()
// 		// We're trying to emulate errgroup here, which always
// 		// executes f even if g.Wait has previously been invoked.
// 		//
// 		// If we weren't constrained by emulating errgroup's behavior,
// 		// it might (?) make sense to panic here, as it's likely a
// 		// programming error to invoke Go after Wait.
//
// 		// We're effectively resetting
// 		g.qCh = make(chan func() error, g.qSize)
// 		g.qClosed = false
// 		g.gCount.Store(1)
// 		g.qMu.Unlock()
//
// 		g.startG()
// 		g.qCh <- f
//
// 		// Thus effectively: if Go is invoked after Wait, we
// 		// emulate the behavior of errgroup (even though this
// 		// can spawn more goroutines that numG).
// 		// g.wg.Add(1)
// 		// go func() {
// 		// 	defer g.wg.Done()
// 		//
// 		// 	if err := f(); err != nil {
// 		// 		g.errOnce.Do(func() {
// 		// 			g.err = err
// 		// 			if g.cancel != nil {
// 		// 				g.cancel()
// 		// 			}
// 		// 		})
// 		// 	}
// 		// }()
//
// 		return
// 	}
//
// 	g.qCh <- f
// 	// Check if we can or should start a new goroutine.
// 	g.maybeStartG()
//
// 	g.qMu.Unlock()
//
// 	return
// }
