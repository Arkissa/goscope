// Package goscope
package goscope

import (
	"context"
	"sync"
)

type Group[F ~func() | ~func(context.Context) error] interface {
	Wait() error
	Go(F)
}

var (
	_ Group[func()] = NewTasksGroup()
	_ Group[func(context.Context) error] = NewErrGroup()
	_ Group[func(context.Context) error] = NewRaceGroup()
)

type noCopy struct{}

type taskGroup struct {
	wg       sync.WaitGroup
	tasks    chan func()
	initOnce sync.Once
}

func NewTasksGroup() *taskGroup {
	return &taskGroup{}
}

func (t *taskGroup) Go(f func()) {
	t.initOnce.Do(func() {
		t.tasks = make(chan func())
	})

	select {
	case t.tasks <- f:
	default:
		t.wg.Add(1)
		go func() {
			defer t.wg.Done()
			if f != nil {
				f()
			}

			for f := range t.tasks {
				f()
			}
		}()
	}
}

func (t *taskGroup) Wait() error {
	if t.tasks != nil {
		close(t.tasks)
	}

	defer func() {
		t.initOnce = sync.Once{}
	}()
	t.wg.Wait()
	return nil
}

type group struct {
	_ noCopy

	cancel func(error)
	ctx    context.Context

	tasks    *taskGroup
	initOnce sync.Once
	errOnce  sync.Once
	err      error
}

func (b *group) init() {
	b.initOnce.Do(func() {
		if b.ctx == nil {
			b.ctx = context.Background()
		}

		b.ctx, b.cancel = context.WithCancelCause(context.Background())
	})
}

func (b *group) WithContext(ctx context.Context) {
	b.ctx = ctx
}

func (b *group) wait() error {
	defer func() {
		b.initOnce = sync.Once{}
		b.errOnce = sync.Once{}
	}()

	b.tasks.Wait()
	return b.err
}

type raceGroup struct {
	group *group
}

func NewRaceGroup() *raceGroup {
	return &raceGroup{
		group: &group{
			tasks: NewTasksGroup(),
		},
	}
}

func (r *raceGroup) WithContext(ctx context.Context) *raceGroup {
	r.group.WithContext(ctx)
	return r
}

func (r *raceGroup) Go(f func(ctx context.Context) error) {
	r.group.init()
	r.group.tasks.Go(func() {
		err := f(r.group.ctx)
		r.group.errOnce.Do(func() {
			r.group.err = err
			r.group.cancel(r.group.err)
		})
	})
}

func (r *raceGroup) Wait() error {
	return r.group.wait()
}

type errGroup struct {
	group *group
}

func NewErrGroup() *errGroup {
	return &errGroup{
		group: &group{
			tasks: NewTasksGroup(),
		},
	}
}

func (g *errGroup) Go(f func(ctx context.Context) error) {
	g.group.init()
	g.group.tasks.Go(func() {
		if err := f(g.group.ctx); err != nil {
			g.group.errOnce.Do(func() {
				g.group.err = err
				g.group.cancel(g.group.err)
			})
		}
	})
}

func (g *errGroup) WithContext(ctx context.Context) *errGroup {
	g.group.WithContext(ctx)
	return g
}

func (g *errGroup) Wait() error {
	return g.group.wait()
}
