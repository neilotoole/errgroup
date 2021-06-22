// Package neilotoole/errgroup is an extension of the sync/errgroup
// concept, and much of the code herein is descended from
// or directly copied from that sync/errgroup code which
// has this header comment:
//
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package errgroup is a drop-in alternative to sync/errgroup but
// limited to N goroutines. In effect, neilotoole/errgroup is
// sync/errgroup but with a worker pool of N goroutines.
package errgroup

import (
	"context"
	"runtime"
	"sync"

	"sync/atomic"
)

// A Group is a collection of goroutines working on subtasks that are part of
// the same overall task.
//
// A zero Group is valid and does not cancel on error.
//
// This Group implementation differs from sync/errgroup in that instead
// of each call to Go spawning a new Go routine, the f passed to Go
// is sent to a queue channel (qCh), and is picked up by one of N
// worker goroutines. The number of goroutines (numG) and the queue
// channel size (qSize) are args to WithContextN. The zero Group and
// the Group returned by WithContext both use default values (the value
// of runtime.NumCPU) for the numG and qSize args. A side-effect of this
// implementation is that the Go method will block while qCh is full: in
// contrast, errgroup.Group's Go method never blocks (it always spawns
// a new goroutine).
type Group struct {
	cancel func()

	wg sync.WaitGroup

	errOnce sync.Once
	err     error

	// numG is the maximum number of goroutines that can be started.
	NumG int

	// qSize is the capacity of qCh, used for buffering funcs
	// passed to method Go.
	QSize int

	// qCh is the buffer used to hold funcs passed to method Go
	// before they are picked up by worker goroutines.
	qCh chan func() error

	// qMu protects qCh.
	qMu sync.Mutex

	// gCount tracks the number of worker goroutines.
	gCount int64
}

// WithContext returns a new Group and an associated Context derived from ctx.
// It is equivalent to WithContextN(ctx, 0, 0).
func WithContext(ctx context.Context) (*Group, context.Context) {
	return WithContextN(ctx, 0, 0) // zero indicates default values
}

// WithContextN returns a new Group and an associated Context derived from ctx.
//
// The derived Context is canceled the first time a function passed to Go
// returns a non-nil error or the first time Wait returns, whichever occurs
// first.
//
// Param numG controls the number of worker goroutines. Param qSize
// controls the size of the queue channel that holds functions passed
// to method Go: while the queue channel is full, Go blocks.
// If numG <= 0, the value of runtime.NumCPU is used; if qSize is
// also <= 0, a qSize of runtime.NumCPU is used.
func WithContextN(ctx context.Context, numG, qSize int) (*Group, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &Group{cancel: cancel, NumG: numG, QSize: qSize}, ctx
}

// Wait blocks until all function calls from the Go method have returned, then
// returns the first non-nil error (if any) from them.
func (g *Group) Wait() error {
	g.qMu.Lock()

	if g.qCh != nil {
		// qCh is typically initialized by the first call to method Go.
		// qCh can be nil if Wait is invoked before the first
		// call to Go, hence this check before we close qCh.
		close(g.qCh)
	}

	// Wait for the worker goroutines to finish.
	g.wg.Wait()

	// All of the worker goroutines have finished,
	// so it's safe to set qCh to nil.
	g.qCh = nil

	g.qMu.Unlock()

	if g.cancel != nil {
		g.cancel()
	}

	return g.err
}

// Go adds the given function to a queue of functions that are called
// by one of g's worker goroutines.
//
// The first call to return a non-nil error cancels the group; its error will be
// returned by Wait.
//
// Go may block while g's qCh is full.
func (g *Group) Go(f func() error) {
	g.qMu.Lock()
	if g.qCh == nil {
		// We need to initialize g.

		// The zero value of numG would mean no worker goroutine
		// would be created, which would be daft.
		// We want the "effective" zero value to be runtime.NumCPU.
		if g.NumG == 0 {
			// Benchmarking has shown that the optimal numG and
			// qSize values depend on the particular workload. In
			// the absence of any other deciding factor, we somewhat
			// arbitrarily default to NumCPU, which seems to perform
			// reasonably in benchmarks. Users that care about performance
			// tuning will use the WithContextN func to specify the numG
			// and qSize args.
			g.NumG = runtime.NumCPU()
			if g.QSize == 0 {
				g.QSize = g.NumG
			}
		}

		g.qCh = make(chan func() error, g.QSize)

		// Being that g.Go has been invoked, we'll need at
		// least one goroutine.
		atomic.StoreInt64(&g.gCount, 1)
		g.startG()

		g.qMu.Unlock()

		g.qCh <- f

		return
	}

	g.qCh <- f

	// Check if we can or should start a new goroutine?
	g.maybeStartG()

	g.qMu.Unlock()

}

// maybeStartG might start a new worker goroutine, if
// needed and allowed.
func (g *Group) maybeStartG() {
	if len(g.qCh) == 0 {
		// No point starting a new goroutine if there's
		// nothing in qCh
		return
	}

	// We have at least one item in qCh. Maybe it's time to start
	// a new worker goroutine?
	if atomic.AddInt64(&g.gCount, 1) > int64(g.NumG) {
		// Nope: not allowed. Starting a new goroutine would put us
		// over the numG limit, so we back out.
		atomic.AddInt64(&g.gCount, -1)
		return
	}

	// It's safe to start a new worker goroutine.
	g.startG()
}

// startG starts a new worker goroutine.
func (g *Group) startG() {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()

		var f func() error

		for {
			// Block until f is received from qCh or
			// the channel is closed.
			f = <-g.qCh
			if f == nil {
				// qCh was closed, time for this goroutine
				// to die.
				return
			}

			if err := f(); err != nil {
				g.errOnce.Do(func() {
					g.err = err
					if g.cancel != nil {
						g.cancel()
					}
				})

				return
			}
		}
	}()
}
