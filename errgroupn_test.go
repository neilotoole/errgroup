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

func newErrgroupZero() grouper {
	return &errgroup.Group{}
}

func newErrgroupnZero() grouper {
	return &errgroupn.Group{}
}

func newErrgroupWithContext() grouper {
	g, _ := errgroup.WithContext(context.Background())
	return g
}
func newErrgroupnWithContext() grouper {
	g, _ := errgroupn.WithContext(context.Background())
	return g
}

func newErrgroupnWithContextN(numG, qSize int) grouper {
	g, _ := errgroupn.WithContextN(context.Background(), numG, qSize)
	return g
}

func TestSmoke(t *testing.T) {
	const (
		loopN = 5
		numG  = 2
		qSize = 2
	)

	testCases := []struct {
		name string
		g    grouper
	}{
		{name: "errgroup_zero", g: newErrgroupZero()},
		{name: "errgroup_wctx", g: newErrgroupWithContext()},
		{name: "errgroupn_zero", g: newErrgroupnZero()},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := tc.g

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
		newG func() grouper
	}{
		{name: "errgroup", newG: newErrgroupZero},
		{name: "errgroupn", newG: newErrgroupnZero},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			actionCh := make(chan struct{}, 1)
			actionMu := &sync.Mutex{}
			actionFn := func() error {
				actionMu.Lock()
				defer actionMu.Unlock()

				doSomething()

				actionCh <- struct{}{}
				return nil
			}

			g := tc.newG()
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
		newG func() grouper
	}{
		{name: "errgroup", newG: newErrgroupZero},
		{name: "errgroupn", newG: newErrgroupnZero},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			actionCh := make(chan struct{}, 1)
			actionMu := &sync.Mutex{}
			actionFn := func() error {
				actionMu.Lock()
				defer actionMu.Unlock()

				doSomething()

				actionCh <- struct{}{}
				return nil
			}

			g := tc.newG()

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

//
// func TestGoAfterWaitEquivalence_0GoThenWait(t *testing.T) {
// 	testCases := []struct {
// 		name string
// 		newG func() grouper
// 	}{
// 		{name: "errgroup", newG: errgroupZero},
// 		{name: "errgroupn", newG: errgroupnZero},
// 	}
//
// 	for _, tc := range testCases {
// 		tc := tc
//
// 		t.Run(tc.name, func(t *testing.T) {
// 			actionCh := make(chan struct{}, 1)
// 			actionMu := &sync.Mutex{}
// 			actionFn := func() error {
// 				actionMu.Lock()
// 				defer actionMu.Unlock()
//
// 				doSomething()
//
// 				actionCh <- struct{}{}
// 				return nil
// 			}
//
// 			g := tc.newG()
// 			g.Go(actionFn)
//
// 			time.Sleep(time.Second)
// 			g.Wait()
// 			if len(actionCh) != 1 {
// 				t.Errorf("actionCh should have one item")
// 			}
// 		})
// 	}
// }
//
// func TestGoAfterWaitEquivalence(t *testing.T) {
// 	actionCh := make(chan struct{}, 1)
// 	actionMu := &sync.Mutex{}
// 	actionFn := func() error {
// 		actionMu.Lock()
// 		defer actionMu.Unlock()
// 		actionCh <- struct{}{}
// 		return nil
// 	}
//
// 	// Test 1: errgroup sanity test
// 	eg := errgroup.Group{}
// 	eg.Go(actionFn)
// 	eg.Wait()
// 	if len(actionCh) != 1 {
// 		t.Errorf("actionCh should have one item")
// 	}
//
// 	<-actionCh // clear actionCh
// 	if len(actionCh) != 0 {
// 		panic("actionCh should be empty")
// 	}
//
// 	// Test 2: eg.Go after eg.Wait
// 	eg = errgroup.Group{}
// 	actionMu.Lock()
// 	eg.Wait()
// 	eg.Go(actionFn)
// 	time.Sleep(time.Second)
// 	actionMu.Unlock()
// 	if len(actionCh) != 0 {
// 		t.Errorf("actionFn should not have been executed according to our understanding")
// 	}
// 	println(len(actionCh))
//
// }

func BenchmarkFibs(b *testing.B) {
	testCases := []struct {
		name string
		fn   func() error
	}{
		{name: "fibsSequential", fn: fibsSequential},
		{name: "fibsErrgroup", fn: fibsErrgroup},
		{name: "fibsErrgroupn_0_0", fn: fibsErrgroupnFunc(0, 0)},
		{name: "fibsErrgroupn_1_1", fn: fibsErrgroupnFunc(1, 1)},
		{name: "fibsErrgroupn_2_2", fn: fibsErrgroupnFunc(2, 2)},
		{name: "fibsErrgroupn_4_16", fn: fibsErrgroupnFunc(4, 16)},
		{name: "fibsErrgroupn_16_4", fn: fibsErrgroupnFunc(16, 4)},
		{name: "fibsErrgroupn_16_16", fn: fibsErrgroupnFunc(16, 4)},
		{name: "fibsErrgroupn_16_32", fn: fibsErrgroupnFunc(16, 32)},
		{name: "fibsErrgroupn_50_100", fn: fibsErrgroupnFunc(50, 100)},
	}

	for _, tc := range testCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := tc.fn()
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

// doSomething spend some time doing something that the compiler
// won't zap away.
func doSomething() error {
	time.Sleep(time.Millisecond)
	return doFib(fibMax)
}

const fibMax = 45

func fibsSequential() error {
	time.Sleep(time.Millisecond)

	for i := 0; i <= fibMax; i++ {
		doFib(i)
	}
	return nil
}

func fibsErrgroup() error {
	g, _ := errgroup.WithContext(context.Background())
	for i := 0; i <= fibMax; i++ {
		i := i
		g.Go(func() error {
			time.Sleep(time.Millisecond)

			doFib(i)
			return nil
		})
	}

	return g.Wait()
}

func fibsErrgroupnFunc(numG, qSize int) func() error {
	return func() error {
		g, _ := errgroupn.WithContextN(context.Background(), numG, qSize)
		for i := 0; i <= fibMax; i++ {
			i := i
			g.Go(func() error {
				time.Sleep(time.Millisecond)
				return doFib(i)
			})
		}

		return g.Wait()
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
