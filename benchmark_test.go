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

// BenchmarkGroup_Short is a (shorter) benchmark of errgroupn.
//
//   go test -run=XXX -bench=BenchmarkGroup_Short -benchtime=1s
func BenchmarkGroup_Short(b *testing.B) {
	cpus := runtime.NumCPU()

	testImpls := []struct {
		name string
		fn   func(tasks, complexity int) error
	}{
		{name: fmt.Sprintf("errgroupn_default_%d_%d", cpus, cpus), fn: doErrgroupnFunc(0, 0)},
		{name: "errgroupn_4_4", fn: doErrgroupnFunc(4, 4)},
		{name: "errgroupn_16_4", fn: doErrgroupnFunc(16, 4)},
		{name: "errgroupn_32_4", fn: doErrgroupnFunc(32, 4)},
		{name: "sync_errgroup", fn: doErrgroup}, // this is the sync/errgroup impl
		{name: "sequential", fn: doSequential},  // for reference, the non-parallel way
	}

	for _, complexity := range []int{5, 20, 40} {
		complexity := complexity

		b.Run(fmt.Sprintf("complexity_%d", complexity), func(b *testing.B) {
			for _, tasks := range []int{10, 50, 250} {
				tasks := tasks

				b.Run(fmt.Sprintf("tasks_%d", tasks), func(b *testing.B) {
					for _, impl := range testImpls {
						impl := impl

						b.Run(impl.name, func(b *testing.B) {
							b.ReportAllocs()
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

// BenchmarkGroup_Long benchmarks errgroupn vs errgroup at
// various configurations and workloads.
//
// The benchmark setup is convoluted, but results in output like so:
//
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_default-16         	   79617	     14827 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_1_0-16             	   63069	     19095 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_1_1-16             	   60290	     19913 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_2_1-16             	   60476	     19621 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_2_2-16             	   85765	     13931 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_2_4-16             	   83857	     14335 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/sync_errgroup-16             	   59840	     19987 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/sequential-16                	   79074	     15193 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_4_2-16             	   85455	     13924 ns/op
// BenchmarkGroup_Long/complexity_2/tasks_10/errgroupn_4_8-16             	   83496	     14323 ns/op
// [...]
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_default-16        	   29162	     41861 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_1_0-16            	   13405	     94965 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_1_1-16            	   14505	     82001 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_2_1-16            	   19932	     58991 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_2_2-16            	   22478	     54035 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_2_4-16            	   24512	     49981 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/sync_errgroup-16            	   26193	     43099 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/sequential-16               	   20118	     60036 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_4_2-16            	   24685	     48527 ns/op
// BenchmarkGroup_Long/complexity_10/tasks_10/errgroupn_4_8-16            	   28038	     42742 ns/op
// [...]
//
// The goal of this benchmark is to generate a bunch of data on
// how errgroupn performs in different configurations {numG, qSize}
// at different workloads {task complexity, num tasks}, also
// including sync/errgroup ("sync_errgroup") and non-parallel
// ("sequential") in the benchmark. The benchmark uses sub-benchmarks
// for: {task complexity, number of tasks, the impl being tested}.
//
// Note that this benchmark takes a long time to run. Typically you'll
// need to set the -timeout flag to 20-60m depending upon your system.
//
//   go test -run=XXX -bench=. -benchtime=1s -timeout=20m
func BenchmarkGroup_Long(b *testing.B) {
	if testing.Short() {
		b.Skipf("This benchmark takes a long time to run")
	}

	b.Log("Go grab lunch, this benchmark takes a long time to run")

	cpus := runtime.NumCPU()

	testImpls := []struct {
		name string
		fn   func(tasks, complexity int) error
	}{
		// These are impls we want for reference
		{name: fmt.Sprintf("errgroupn_default_%d_%d", cpus, cpus), fn: doErrgroupnFunc(0, 0)},
		{name: "errgroupn_1_0", fn: doErrgroupnFunc(1, 0)},
		{name: "errgroupn_1_1", fn: doErrgroupnFunc(1, 1)},
		{name: "errgroupn_2_1", fn: doErrgroupnFunc(2, 1)},
		{name: "errgroupn_2_2", fn: doErrgroupnFunc(2, 2)},
		{name: "errgroupn_2_4", fn: doErrgroupnFunc(2, 4)},
		{name: "sync_errgroup", fn: doErrgroup}, // this is the sync/errgroup impl
		{name: "sequential", fn: doSequential},  // for reference, the non-parallel way
	}

	for _, numG := range []int{4, 16, 64, 512} {
		for _, qSize := range []int{2, 8, 64} {
			testImpls = append(testImpls, struct {
				name string
				fn   func(tasks int, complexity int) error
			}{name: fmt.Sprintf("errgroupn_%d_%d", numG, qSize), fn: doErrgroupnFunc(numG, qSize)})
		}
	}

	for _, complexity := range []int{2, 10, 20, 40} {
		complexity := complexity

		b.Run(fmt.Sprintf("complexity_%d", complexity), func(b *testing.B) {
			for _, tasks := range []int{10, 100, 500 /*, 1000, 5000*/} {
				tasks := tasks

				b.Run(fmt.Sprintf("tasks_%d", tasks), func(b *testing.B) {
					for _, impl := range testImpls {
						impl := impl

						b.Run(impl.name, func(b *testing.B) {
							b.ReportAllocs()
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
// compiler won't zap away. The complexity param controls
// how long the work takes.
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
