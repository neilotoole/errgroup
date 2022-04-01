package errgroup_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	errgroupn "github.com/neilotoole/errgroup"
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

func newErrgroupnWithContextN(numG, qSize int) func() (grouper, context.Context) {
	return func() (grouper, context.Context) {
		return errgroupn.WithContextN(context.Background(), numG, qSize)
	}
}

func TestGroup(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup_zero", newG: newErrgroupZero},
		{name: "errgroup_wctx", newG: newErrgroupWithContext},
		{name: "errgroupn_zero", newG: newErrgroupnZero},
		{name: "errgroupn_wctx", newG: newErrgroupnWithContext},
		{name: "errgroupn_wctx_0_0", newG: newErrgroupnWithContextN(0, 0)},
		{name: "errgroupn_wctx_1_0", newG: newErrgroupnWithContextN(1, 0)},
		{name: "errgroupn_wctx_1_1", newG: newErrgroupnWithContextN(1, 1)},
		{name: "errgroupn_wctx_4_16", newG: newErrgroupnWithContextN(4, 16)},
		{name: "errgroupn_wctx_16_4", newG: newErrgroupnWithContextN(16, 4)},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g, _ := tc.newG()

			vals := make([]int, fibMax)
			for i := 0; i < fibMax; i++ {
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
			if !equalInts(vals, fibVals[fibMax-1]) {
				t.Errorf("vals (%d) incorrect: %v  |  %v", fibMax, vals, fibVals[fibMax])
			}

			// Let's do this a second time to verify that g.Go continues
			// to work after the first call to g.Wait
			vals = make([]int, fibMax)
			for i := 0; i < fibMax; i++ {
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
			if !equalInts(vals, fibVals[fibMax-1]) {
				t.Errorf("vals (%d) incorrect: %v  |  %v", fibMax, vals, fibVals[fibMax])
			}
		})

	}

}

func TestGroupWithErrors(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup_zero", newG: newErrgroupZero},
		{name: "errgroup_wctx", newG: newErrgroupWithContext},
		{name: "errgroupn_zero", newG: newErrgroupnZero},
		{name: "errgroupn_wctx", newG: newErrgroupnWithContext},
		{name: "errgroupn_wctx_0_0", newG: newErrgroupnWithContextN(0, 0)},
		{name: "errgroupn_wctx_1_0", newG: newErrgroupnWithContextN(1, 0)},
		{name: "errgroupn_wctx_1_1", newG: newErrgroupnWithContextN(1, 1)},
		{name: "errgroupn_wctx_4_16", newG: newErrgroupnWithContextN(4, 16)},
		{name: "errgroupn_wctx_16_4", newG: newErrgroupnWithContextN(16, 4)},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f := func() {
				g, _ := tc.newG()

				for i := 0; i < 1000; i++ {
					i := i
					g.Go(func() error {
						// return an error for every 3
						if i%3 == 0 {
							return errors.New("sample error")
						}

						return nil
					})
				}

				if g.Wait() == nil {
					t.Error("Wait should return an error but did not")
				}
			}

			// this may cause a deadlock, so running test with a timeout
			testTimeout := 10 * time.Second
			if err := mustRunInTime(testTimeout, f); err != nil {
				t.Errorf("mustRunInTime failed with error: %v", err)
			}
		})
	}
}

func TestEquivalence_GoWaitThenGoAgain(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup_zero", newG: newErrgroupZero},
		{name: "errgroup_wctx", newG: newErrgroupWithContext},
		{name: "errgroupn_zero", newG: newErrgroupnZero},
		{name: "errgroupn_wctx", newG: newErrgroupnWithContext},
		{name: "errgroupn_wctx_16_4", newG: newErrgroupnWithContextN(16, 4)},
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

				_ = doWork(gctx, 10)

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

func TestEquivalence_WaitThenGo(t *testing.T) {
	testCases := []struct {
		name string
		newG func() (grouper, context.Context)
	}{
		{name: "errgroup_zero", newG: newErrgroupZero},
		{name: "errgroup_wctx", newG: newErrgroupWithContext},
		{name: "errgroupn_zero", newG: newErrgroupnZero},
		{name: "errgroupn_wctx", newG: newErrgroupnWithContext},
		{name: "errgroupn_wctx_16_4", newG: newErrgroupnWithContextN(16, 4)},
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

				_ = doWork(gctx, 10)

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

// mustRunInTime returns an error if execution of a function
// takes more than the timeout set
func mustRunInTime(d time.Duration, f func()) error {
	c := make(chan struct{}, 1)

	// Run your long running function in it's own goroutine and pass back it's
	// response into our channel.
	go func() {
		f()
		c <- struct{}{}
	}()

	// Listen on our channel AND a timeout channel - which ever happens first.
	select {
	case <-c:
		return nil
	case <-time.After(d):
		return errors.New("timeout")
	}
}
