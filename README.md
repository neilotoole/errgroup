# errgroupn
`errgroupn` is a drop-in alternative to Go's `sync/errgroup` but limited
to N goroutines. In effect, `errgroupn` is `errgroup` but with a worker pool
of N goroutines. The exported API is identical but for an additional
constructor function `WithContextN`, which allows the caller
to specify the maximum number of goroutines (`numG`) and the capacity
of the queue channel (`qSize`) used to hold work before being picked
up by a worker goroutine. The zero `Group` and the `WithContext`
constructor both return a `Group` with `numG` equal to `runtime.NumCPU` 




## Open Question
Why require an explicit `qSize` limit?

If the number of calls to `errgroupn/Group.Go` results in `qCh` becoming
full, the `Go` method will block until worker goroutines relieve `qCh`.
This behavior is at odds with `sync/errgroup.Go`, which doesn't block.
Given that `errgroupn` aims to be as much of a behaviorally identical
"drop-in" alternative to `errgroup` as possible

The capacity of `qCh` is controlled by `qSize`, but it's probable an
alternative implementation could be built that uses a (growable) slice
acting, if `qCh` is full, as a buffer for functions passed to `Go`.
Consideration of this issue led me to this [issue](https://github.com/golang/go/issues/20352)
regarding _unlimited capacity channels_, or perhaps better characterized
in this particular case as "_growable capacity channels_". If such a
feature existed in the language, it's possible that `errgroupn` might
have taken advantage of it, at least in the first-pass release (benchmarking
etc notwithstanding). 