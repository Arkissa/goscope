## What is structured concurrency.
Structured concurrency 是通过给线程或用户线程添加控制流，让线程之间拥有明确的父子关系和生命周期的约束，这样做带来的好处有：
1. 可预测和可控的生命周期, 父线程可取消和等待子线程，避免孤儿线程，保证线程得以释放。
2. 线程具备任务边界作用域。
3. 子线程的错误/异常可以向上传递给父线程。

换句话说，线程不再独立存在，而是围绕某个任务（例如生产者或业务单元）进行调度和管理，所有创建的并发都聚焦于该任务的作用域，从而保证结构化和可控性。

## Why Go need structured concurrency.
在 Go 中是直接使用 `go` 语句创建用户线程也就是协程，但这是一个非常低级的 api ，缺少以下特性：
1. 不具备协程之间的父子关系.
2. 子协程出现错误或 panic 父协程无法感知.
3. 每个协程都是全局独立的，不具备任务作用域，其中的生命周期是不可预测的。

正因为缺少这些特性让 Go 的并发编程非常容易出现不可控行为，孤儿协程更是 Go 最常见的内存泄露原因之一，这些问题这里有一篇著名 blog 强调了只使用 `go` 语句会带来的问题:
- [Notes on structured concurrency, or: Go statement considered harmful](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/)

结构化并发能有效的将一组 goroutine 的生命周期控制起来，goroutine 不再是全局的黑盒子，而是控制在一个分组中针对某一项任务去执行，创建任务的父协程将可以控制子协程的生命周期，等待子协程的结果，也能感知子协程的错误。
越来越多的语言都强调支持结构化并发，甚至都内置到自己的语言语法中或者标准库，支持结构化并发的语言如下:
- [Kotlin coroutines](https://kotlinlang.org/docs/coroutines-guide.html)
- [Java JEP 505](https://openjdk.org/jeps/505)
- [Haskell Async Run IO operations asynchronously and wait for their results](https://hackage.haskell.org/package/async)
- [Moonbit Asynchronous programming library for MoonBit](https://github.com/moonbitlang/async)
- [Structured Concurrency in C](https://www.250bpm.com/p/structured-concurrency)

Go 作为一个天生为并发而生的语言不应该落后，也应该拥有自己的结构化并发。

## How to do.
本库是 Go 的一个结构化并发的轻量实现，目的是很好的嵌入到业务框架中，方便的对业务的子业务构建对应的协程组去执行任务，将所有发起的协程都具备结构性。一些例子：
<table>
<tr>
<th><code>TaskGroup</code></th>
</tr>
<tr>
<td>

```go
func main() {

    // TaskGroup will reuse coroutines as much as possible.
    tg := goscope.NewTaskGroup()
    tg.Go(func() {
      // do something
    })

    tg.Wait()

}
```
</td>
</tr>
</table>

<table>
<tr>
<th><code>RaceGroup</code></th>
</tr>
<tr>
<td>

```go
func main() {

    // Happy Eyeballs
    rg := goscope.NewRaceGroup()
    for _, addr := range addrs {
      rg.Go(func(ctx context.Context) error {
        conn, err := new(net.Dialer).DialContext(ctx, "tcp", addr)
        if err != nil {
          return err
        }
        defer conn.Close()

        return nil
      })

      time.Sleep(250 * time.Millisecond)
    }

    err := rg.Wait()
    if err != nil {
      panic(err)
    }

}
```
</td>
</tr>
</table>

## TODO
- [ ] Catcher child panic on parent.
- [ ] Stream Parallelism
- [ ] Child return value

