package goscope

import (
	"context"
	"sync"
)

type result[T any] struct {
	t   T
	err error
}

func (r *result[T]) Value() T {
	return r.t
}

func (r *result[T]) Err() error {
	return r.err
}

type LazyInit[T any] struct {
	f    func(context.Context) (T, error)
	ch   chan *result[T]
	once sync.Once
}

func NewLazyInit[T any](f func(context.Context) (T, error)) *LazyInit[T] {
	return &LazyInit[T]{
		f:   f,
		ch:  make(chan *result[T], 1),
	}
}

func (la *LazyInit[T]) Wait(ctx context.Context) (T, error) {
	la.once.Do(func() {
		go func() {
			r, err := la.f(ctx)
			la.ch <- &result[T]{
				t:   r,
				err: err,
			}
		}()
	})

	res := <-la.ch
	la.ch <- res
	return res.Value(), res.Err()
}
