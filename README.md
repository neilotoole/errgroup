# errgroupn
`errgroupn` is a drop-in alternative to Go's `sync/errgroup` but limited
to N goroutines. This is useful for interaction with rate-lmited
APIs, databases, and the like.

## Overview
In effect, `errgroupn` is `errgroup` but with a worker pool
of `N` goroutines. The exported API is identical but for an additional
function `WithContextN`, which allows the caller
to specify the maximum number of goroutines (`numG`) and the capacity
of the queue channel (`qSize`) used to hold work before it is picked
up by a worker goroutine. The zero `Group` and the `WithContext`
function both return a `Group` with `numG` and `qSize` equal to `runtime.NumCPU`.


## Usage
The exported API of this package mirrors the `sync/errgroup` package.
The only change needed is the import path of the package, from:

```go
import (
  "golang.org/x/sync/errgroup"
)
```

to

```go
import (
  "errgroup" "github.com/neilotoole/errgroupn"
)
```

Then use in the normal manner.

```go
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error {
    // do something
    return nil
})

err := g.Wait()
```

Many users will have no need to tweak the `numG` and `qCh` params.

However, benchmarking may suggest particular values for your workload.
For that you'll need `WithContextN`:

```go
numG, qSize := 8, 4
g, ctx := errgroup.WithContextN(ctx, numG, qSize)

```

## Performance
The motivation for creating `errgroupn` was to provide rate-limiting while
maintaining the lovely `sync/errgroup` semantics. Sacrificing some
performance vs `sync/errgroup` was assumed. However, benchmarking
suggests that `errgroupn` can be more effective than `sync/errgroup` 
when tuned for a specific workload.

Below is a selection of benchmark results. How to read this: a workload is `X` tasks
of `Y` complexity. The workload is executed via `sync/errgroup`; a
non-parallel implementation (`sequential`); and various (`numG`, `qCh`)
configurations of `errgroupn`.

```
BenchmarkGroup_Short/complexity_5/tasks_50/errgroupn_default_16_16-16         	   25574	     46867 ns/op	     688 B/op	      12 allocs/op
BenchmarkGroup_Short/complexity_5/tasks_50/errgroupn_4_4-16                   	   24908	     48926 ns/op	     592 B/op	      12 allocs/op
BenchmarkGroup_Short/complexity_5/tasks_50/errgroupn_16_4-16                  	   24895	     48313 ns/op	     592 B/op	      12 allocs/op
BenchmarkGroup_Short/complexity_5/tasks_50/errgroupn_32_4-16                  	   24853	     48284 ns/op	     592 B/op	      12 allocs/op
BenchmarkGroup_Short/complexity_5/tasks_50/sync_errgroup-16                   	   18784	     65826 ns/op	    1858 B/op	      55 allocs/op
BenchmarkGroup_Short/complexity_5/tasks_50/sequential-16                      	   10000	    111483 ns/op	       0 B/op	       0 allocs/op

BenchmarkGroup_Short/complexity_20/tasks_50/errgroupn_default_16_16-16        	    3745	    325993 ns/op	    1168 B/op	      27 allocs/op
BenchmarkGroup_Short/complexity_20/tasks_50/errgroupn_4_4-16                  	    5186	    227034 ns/op	    1072 B/op	      27 allocs/op
BenchmarkGroup_Short/complexity_20/tasks_50/errgroupn_16_4-16                 	    3970	    312816 ns/op	    1076 B/op	      27 allocs/op
BenchmarkGroup_Short/complexity_20/tasks_50/errgroupn_32_4-16                 	    3715	    320757 ns/op	    1073 B/op	      27 allocs/op
BenchmarkGroup_Short/complexity_20/tasks_50/sync_errgroup-16                  	    2739	    432093 ns/op	    1862 B/op	      55 allocs/op
BenchmarkGroup_Short/complexity_20/tasks_50/sequential-16                     	    2306	    520947 ns/op	       0 B/op	       0 allocs/op

BenchmarkGroup_Short/complexity_40/tasks_250/errgroupn_default_16_16-16       	     354	   3602666 ns/op	    1822 B/op	      47 allocs/op
BenchmarkGroup_Short/complexity_40/tasks_250/errgroupn_4_4-16                 	     420	   2468605 ns/op	    1712 B/op	      47 allocs/op
BenchmarkGroup_Short/complexity_40/tasks_250/errgroupn_16_4-16                	     334	   3581349 ns/op	    1716 B/op	      47 allocs/op
BenchmarkGroup_Short/complexity_40/tasks_250/errgroupn_32_4-16                	     310	   3890316 ns/op	    1712 B/op	      47 allocs/op
BenchmarkGroup_Short/complexity_40/tasks_250/sync_errgroup-16                 	     253	   4740462 ns/op	    8303 B/op	     255 allocs/op
BenchmarkGroup_Short/complexity_40/tasks_250/sequential-16                    	     200	   5924693 ns/op	       0 B/op	       0 allocs/op
```

The overall impression is that that `errgroupn` can provide higher
throughput than `sync/errgroup` for these (CPU-intensive) workloads,
sometimes significantly so. As always, these benchmark results should
not be taken as gospel: your results may vary.


## Design Note
Why require an explicit `qSize` limit?

If the number of calls to `Group.Go` results in `qCh` becoming
full, the `Go` method will block until worker goroutines relieve `qCh`.
This behavior is in constrast to `sync/errgroup.Go`, which doesn't block.
While `errgroupn` aims to be as much of a behaviorally identical
"drop-in" alternative to `errgroup` as possible, this blocking behavior
is an accepted deviation.

Relatedly: the capacity of `qCh` is controlled by `qSize`, but it's probable an
alternative implementation could be built that uses a (growable) slice
acting, if `qCh` is full, as a buffer for functions passed to `Go`.
Consideration of this design led to this [issue](https://github.com/golang/go/issues/20352)
regarding _unlimited capacity channels_, or perhaps better characterized
in this particular case as "_growable capacity channels_". If such a
feature existed in the language, it's possible that `errgroupn` might
have taken advantage of it, at least in the first-pass release (benchmarking
etc notwithstanding). 