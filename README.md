# errgroupn
errgroupn is Go's sync.errgroup with goroutine worker limits


## Open Question
Why require an explicit `qSize` limit?

If the quantity of calls to `errgroupn/Group.Go` results in `qCh` becoming
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