// Package errgroupn is an extension of the sync.errgroup
// concept, and the code herein is directly descended from
// that sync.errgroup code which has this header comment:
//
// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errgroupn_test

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/errgroupn"
)

// grouper is an abstraction of errgroup.Group's exported methodset.
type grouper interface {
	Go(f func() error)
	Wait() error
}

var (
	_ grouper = &errgroup.Group{}
	_ grouper = &errgroupn.Group{}
)

func newErrgroupZero() (grouper, context.Context) {
	return &errgroup.Group{}, context.Background()
}

func newErrgroupnZero() (grouper, context.Context) {
	return &errgroupn.Group{}, context.Background()
}

func newErrgroupWithContext() (grouper, context.Context) {
	return errgroup.WithContext(context.Background())
}
func newErrgroupnWithContext() (grouper, context.Context) {
	return errgroupn.WithContext(context.Background())
}

func newErrgroupnWithContextN(numG, qSize int) (grouper, context.Context) {
	return errgroupn.WithContextN(context.Background(), numG, qSize)
}

func TestSmoke(t *testing.T) {
	const (
		loopN = 5
		numG  = 2
		qSize = 2
	)

	testCases := []struct {
		name string
		g    func() (grouper, context.Context)
	}{
		{name: "errgroup_zero", g: newErrgroupZero},
		{name: "errgroup_wctx", g: newErrgroupWithContext},
		{name: "errgroupn_zero", g: newErrgroupnZero},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g, _ := tc.g()

			vals := make([]int, loopN)
			for i := 0; i < loopN; i++ {
				i := i
				g.Go(func() error {
					vals[i] = fib(i)
					return nil
				})
			}

			err := g.Wait()
			if err != nil {
				t.Error(err)
			}
			if !equalInts(vals, fibVals[loopN-1]) {
				t.Errorf("vals (%d) incorrect: %v  |  %v", loopN, vals, fibVals[loopN])
			}

			println("\n\n\n **** doing it again ****\n\n\n")

			vals = make([]int, loopN)
			for i := 0; i < loopN; i++ {
				i := i
				g.Go(func() error {
					vals[i] = fib(i)
					return nil
				})
			}

			err = g.Wait()
			if err != nil {
				t.Error(err)
			}
			if !equalInts(vals, fibVals[loopN-1]) {
				t.Errorf("vals (%d) incorrect: %v  |  %v", loopN, vals, fibVals[loopN])
			}
		})

	}

}

func TestEquivalence_ZeroGroup_GoWaitThenGoAgain(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup", newG: newErrgroupZero},
		{name: "errgroupn", newG: newErrgroupnZero},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			g, gctx := tc.newG()

			actionCh := make(chan struct{}, 1)
			actionMu := &sync.Mutex{}
			actionFn := func() error {
				actionMu.Lock()
				defer actionMu.Unlock()

				doWork(gctx, 10)

				actionCh <- struct{}{}
				return nil
			}

			g.Go(actionFn)

			err := g.Wait()
			if err != nil {
				t.Error(err)
			}
			if len(actionCh) != 1 {
				t.Errorf("actionCh should have one item")
			}

			// drain actionCh
			<-actionCh

			g.Go(actionFn)

			err = g.Wait()
			if err != nil {
				t.Error(err)
			}
			if len(actionCh) != 1 {
				t.Errorf("actionCh should have one item")
			}
		})
	}
}

func TestEquivalence_ZeroGroup_WaitThenGo(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup", newG: newErrgroupZero},
		{name: "errgroupn", newG: newErrgroupnZero},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			g, gctx := tc.newG()

			actionCh := make(chan struct{}, 1)
			actionMu := &sync.Mutex{}
			actionFn := func() error {
				actionMu.Lock()
				defer actionMu.Unlock()

				doWork(gctx, 10)

				actionCh <- struct{}{}
				return nil
			}

			time.Sleep(time.Second)
			err := g.Wait()
			if err != nil {
				t.Error(err)
			}
			if len(actionCh) != 0 {
				t.Errorf("actionCh should have zero items")
			}

			g.Go(actionFn)

			time.Sleep(time.Second)
			err = g.Wait()
			if err != nil {
				t.Error(err)
			}
			if len(actionCh) != 1 {
				t.Errorf("actionCh should have one item")
			}
		})
	}
}

// fibVals holds computed values of the fibonacci sequence.
// Each row holds the fib sequence for that row's index. That is,
// the first few rows look like:
//
//   [0]
//   [0 1]
//   [0 1 1]
//   [0 1 1 2]
//   [0 1 1 2 3]
var fibVals [][]int

const fibMax = 50

func init() {
	fibVals = make([][]int, fibMax)
	for i := 0; i < fibMax; i++ {
		fibVals[i] = make([]int, i+1)
		if i == 0 {
			fibVals[0][0] = 0
			continue
		}
		copy(fibVals[i], fibVals[i-1])
		fibVals[i][i] = fib(i)
	}

}

// fibFill fills each element fibs[i] with fib(i).
func fibFill(fibs []int) error {
	for i := range fibs {
		fibs[i] = fib(i)
	}
	return nil
}

func doFib(n int) error {
	f := fib(n)
	runtime.KeepAlive(f) // Prevent compiler optimization for benchmarking
	// fmt.Fprintln(ioutil.Discard, fib(n))
	return nil
}

// fib returns the fibonacci sequence of n.
func fib(n int) int {
	a, b, temp := 0, 1, 0
	for i := 0; i < n; i++ {
		temp = a
		a = b
		b = temp + a
	}
	return a
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
