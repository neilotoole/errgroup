package errgroupn_test

import (
	"context"
	"crypto/sha512"
	"fmt"
	"runtime"
	"testing"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/errgroupn"
)

func BenchmarkErrgroupn(b *testing.B) {
	impls := []struct {
		name string
		fn   func(tasks, complexity int) error
	}{
		// {name: "errgroupn_1_1", fn: doErrgroupnFunc(1, 1)},
		// {name: "errgroupn_1_2", fn: doErrgroupnFunc(1, 2)},
		{name: "errgroupn_2_1", fn: doErrgroupnFunc(2, 1)},
		{name: "errgroupn_2_2", fn: doErrgroupnFunc(2, 2)},
		{name: "errgroupn_2_4", fn: doErrgroupnFunc(2, 4)},
		{name: "errgroupn_2_16", fn: doErrgroupnFunc(2, 16)},
		{name: "errgroupn_4_2", fn: doErrgroupnFunc(4, 2)},
		{name: "errgroupn_4_4", fn: doErrgroupnFunc(4, 4)},
		{name: "errgroupn_4_16", fn: doErrgroupnFunc(4, 16)},
		{name: "errgroupn_16_1", fn: doErrgroupnFunc(16, 1)},
		{name: "errgroupn_16_2", fn: doErrgroupnFunc(16, 2)},
		{name: "errgroupn_16_4", fn: doErrgroupnFunc(16, 4)},
		{name: "errgroupn_16_16", fn: doErrgroupnFunc(16, 16)},
		{name: "errgroupn_16_32", fn: doErrgroupnFunc(16, 32)},
		{name: "errgroupn_16_64", fn: doErrgroupnFunc(16, 64)},
		{name: "errgroupn_32_1", fn: doErrgroupnFunc(32, 1)},
		{name: "errgroupn_32_2", fn: doErrgroupnFunc(32, 2)},
		{name: "errgroupn_32_4", fn: doErrgroupnFunc(32, 4)},
		{name: "errgroupn_32_16", fn: doErrgroupnFunc(32, 16)},
		{name: "errgroupn_32_32", fn: doErrgroupnFunc(32, 32)},
		{name: "errgroupn_32_64", fn: doErrgroupnFunc(32, 64)},
		{name: "errgroupn_64_1", fn: doErrgroupnFunc(64, 1)},
		{name: "errgroupn_64_2", fn: doErrgroupnFunc(64, 2)},
		{name: "errgroupn_64_4", fn: doErrgroupnFunc(64, 4)},
		{name: "errgroupn_64_16", fn: doErrgroupnFunc(64, 16)},
		{name: "errgroupn_64_32", fn: doErrgroupnFunc(64, 32)},
		{name: "errgroupn_64_64", fn: doErrgroupnFunc(64, 64)},
		{name: "errgroupn_1024_2", fn: doErrgroupnFunc(1024, 2)},
		{name: "errgroupn_1024_4", fn: doErrgroupnFunc(1024, 4)},
		{name: "errgroupn_1024_16", fn: doErrgroupnFunc(1024, 16)},
		{name: "errgroupn_1024_1024", fn: doErrgroupnFunc(1024, 1024)},
		{name: "errgroupn_default", fn: doErrgroupnFunc(0, 0)},
		{name: "sync_errgroup", fn: doErrgroup},
		{name: "sequential", fn: doSequential},
	}

	// var workloads []struct {
	// 	complexity int
	// 	tasks      int
	// }
	//
	// // for complexity := range []int{2, 10, 25, 50, 100} {
	// for _, complexity := range []int{2, 10, 20} {
	// 	// for tasks := range []int{10, 100, 500, 1000, 5000} {
	// 	for _, tasks := range []int{10, 100, 500} {
	// 		workloads = append(workloads, struct {
	// 			complexity int
	// 			tasks      int
	// 		}{complexity: complexity, tasks: tasks})
	// 	}
	// }

	for _, complexity := range []int{2, 10, 20, 40} {
		complexity := complexity

		b.Run(fmt.Sprintf("complexity_%d", complexity), func(b *testing.B) {
			for _, tasks := range []int{10, 100, 500, 1000, 5000} {
				tasks := tasks

				b.Run(fmt.Sprintf("tasks_%d", tasks), func(b *testing.B) {
					for _, impl := range impls {
						impl := impl
						b.Run(impl.name, func(b *testing.B) {
							// b.ReportAllocs()

							for i := 0; i < b.N; i++ {
								err := impl.fn(complexity, tasks)
								if err != nil {
									b.Error(err)
								}
							}
						})
					}
				})
			}
		})
	}
}

// doWork spends some time doing something that the
// compiler won't zap away.
func doWork(ctx context.Context, complexity int) error {
	const text = `In Xanadu did Kubla Khan
A stately pleasure-dome decree:
Where Alph, the sacred river, ran
Through caverns measureless to man
   Down to a sunless sea.
So twice five miles of fertile ground
With walls and towers were girdled round;
And there were gardens bright with sinuous rills,
Where blossomed many an incense-bearing tree;
And here were forests ancient as the hills,
Enfolding sunny spots of greenery.
`

	b := []byte(text)
	var res [sha512.Size256]byte

	for i := 0; i < complexity; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		res = sha512.Sum512_256(b)
		b = res[0:]
	}
	runtime.KeepAlive(b)
	return nil
}

func doSequential(complexity, tasks int) error {
	ctx := context.Background()
	var err error

	for i := 0; i <= tasks; i++ {
		err = doWork(ctx, complexity)
		if err != nil {
			break
		}
	}
	return err
}

func doErrgroup(complexity, tasks int) error {
	g, ctx := errgroup.WithContext(context.Background())
	for i := 0; i <= tasks; i++ {
		g.Go(func() error {
			return doWork(ctx, complexity)
		})
	}

	return g.Wait()
}

func doErrgroupnFunc(numG, qSize int) func(int, int) error {
	return func(tasks, complexity int) error {
		g, ctx := errgroupn.WithContextN(context.Background(), numG, qSize)
		for i := 0; i <= tasks; i++ {
			g.Go(func() error {
				return doWork(ctx, complexity)
			})
		}

		return g.Wait()
	}
}

// func BenchmarkSomething(b *testing.B) {
// 	b.ReportAllocs()
//
// 	ctx := context.Background()
//
// 	for i := 0; i < b.N; i++ {
// 		err := doWork(ctx)
// 		if err != nil {
// 			b.Error(err)
// 		}
// 	}
// }
