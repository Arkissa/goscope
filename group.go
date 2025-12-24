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
	_ Group[func()]                      = NewTasksGroup()
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

func (g *group) init() {
	g.initOnce.Do(func() {
		if g.ctx == nil {
			g.ctx = context.Background()
		}

		g.ctx, g.cancel = context.WithCancelCause(g.ctx)
	})
}

func (g *group) ErrOnceFunc(f func() error) {
	g.errOnce.Do(func() {
		g.err = f()
	})
}

func (g *group) WithContext(ctx context.Context) {
	g.ctx = ctx
}

func (g *group) Context() context.Context {
	return g.ctx
}

func (g *group) Cancel(err error) {
	g.cancel(err)
}

func (g *group) Wait() error {
	defer func() {
		g.initOnce = sync.Once{}
		g.errOnce = sync.Once{}
	}()

	g.tasks.Wait()
	return g.err
}

func (g *group) Go(f func()) {
	g.tasks.Go(f)
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
	r.group.Go(func() {
		err := f(r.group.Context())
		r.group.ErrOnceFunc(func() error {
			r.group.Cancel(err)
			return err
		})
	})
}

func (r *raceGroup) Wait() error {
	return r.group.Wait()
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
	g.group.Go(func() {
		if err := f(g.group.Context()); err != nil {
			g.group.ErrOnceFunc(func() error {
				g.group.Cancel(err)
				return err
			})
		}
	})
}

func (g *errGroup) WithContext(ctx context.Context) *errGroup {
	g.group.WithContext(ctx)
	return g
}

func (g *errGroup) Wait() error {
	return g.group.Wait()
}
