package goscope_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Arkissa/goscope"
)

func TestTaskGroup(t *testing.T) {
	tests := []struct {
		name  string
		tasks int
		delay time.Duration
		want  int32
	}{
		{name: "runs all tasks", tasks: 8, delay: 5 * time.Millisecond, want: 8},
		{name: "handles many tasks", tasks: 50, delay: 0, want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := goscope.NewTasksGroup()
			var counter atomic.Int32

			for i := 0; i < tt.tasks; i++ {
				tg.Go(func() {
					if tt.delay > 0 {
						time.Sleep(tt.delay)
					}
					counter.Add(1)
				})
			}

			tg.Wait()

			if got := counter.Load(); got != tt.want {
				t.Fatalf("executed %d tasks, want %d", got, tt.want)
			}
		})
	}
}

func TestRaceGroup(t *testing.T) {
	var ErrFaild = errors.New("failed running")
	tests := []struct {
		name    string
		do      func(goscope.Group[func(context.Context) error]) error
		wantErr bool
	}{
		{
			name: "fast error cancels slower task",
			do: func(rg goscope.Group[func(context.Context) error]) error {
				rg.Go(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(200 * time.Millisecond):
						return nil
					}
				})

				rg.Go(func(ctx context.Context) error {
					time.Sleep(100 * time.Millisecond)
					return ErrFaild
				})

				return rg.Wait()
			},
			wantErr: true,
		},
		{
			name: "fast success cancels others task",
			do: func(rg goscope.Group[func(context.Context) error]) error {
				rg.Go(func(ctx context.Context) error { return nil })
				rg.Go(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(200 * time.Millisecond):
						return ErrFaild
					}
				})

				return rg.Wait()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.do(goscope.NewRaceGroup())

			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Wait() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Wait() succeeded unexpectedly")
			}
		})
	}
}

func TestErrGroup(t *testing.T) {
	var (
		ErrFaild = errors.New("faild")
		ErrSecondFaild = errors.New("second faild")
	)
	tests := []struct {
		name         string
		do           func() error
		wantErr      bool
		wantErrValue error
	}{
		{
			name: "captures first error and cancel others",
			do: func() error {
				eg := goscope.NewErrGroup().WithContext(context.Background())
				eg.Go(func(ctx context.Context) error { return ErrFaild })
				eg.Go(func(ctx context.Context) error {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(200 * time.Millisecond):
						return nil
					}
				})
				return eg.Wait()
			},
			wantErr: true,
			wantErrValue: ErrFaild,
		},
		{
			name: "all success returns nil",
			do: func() error {
				eg := goscope.NewErrGroup()
				eg.Go(func(ctx context.Context) error { return nil })
				eg.Go(func(ctx context.Context) error { return nil })
				return eg.Wait()
			},
			wantErr: false,
		},
		{
			name: "captures first error but delay",
			do: func() error {
				eg := goscope.NewErrGroup()
				eg.Go(func(ctx context.Context) error { return nil })
				eg.Go(func(ctx context.Context) error {
					time.Sleep(200 * time.Millisecond)
					return ErrFaild
				})
				return eg.Wait()
			},
			wantErr: true,
			wantErrValue: ErrFaild,
		},
		{
			name: "captures first error",
			do: func() error {
				eg := goscope.NewErrGroup()
				eg.Go(func(ctx context.Context) error { return ErrFaild })
				eg.Go(func(ctx context.Context) error {
					time.Sleep(200 * time.Millisecond)
					return ErrSecondFaild
				})
				return eg.Wait()
			},
			wantErr: true,
			wantErrValue: ErrFaild,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.do()

			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Wait() failed: %v", gotErr)
				}

				if !errors.Is(gotErr, tt.wantErrValue) {
					t.Errorf("Wait() want: %v, but got: %v", tt.wantErrValue, gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Wait() succeeded unexpectedly")
			}
		})
	}
}
