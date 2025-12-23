package goscope

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
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
			tg := NewTasksGroup()
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
	tests := []struct {
		name    string
		do      func(*raceGroup) error
		wantErr bool
	}{
		{
			name: "fast error cancels slower task",
			do: func(rg *raceGroup) error {
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
					return errors.New("failed running")
				})

				return rg.Wait()
			},
			wantErr: true,
		},
		{
			name: "fast success cancels others",
			do: func(rg *raceGroup) error {
				rg.Go(func(ctx context.Context) error { return nil })
				rg.Go(func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				})

				return rg.Wait()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.do(NewRaceGroup())

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
	tests := []struct {
		name    string
		setup   func(*errGroup) error
		wantErr bool
	}{
		{
			name: "captures first error and cancels",
			setup: func(eg *errGroup) error {
				eg.Go(func(ctx context.Context) error { return errors.New("fail") })
				eg.Go(func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				})
				return eg.Wait()
			},
			wantErr: true,
		},
		{
			name: "all success returns nil",
			setup: func(eg *errGroup) error {
				eg.Go(func(ctx context.Context) error { return nil })
				eg.Go(func(ctx context.Context) error { return nil })
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eg := NewErrGroup()
			gotErr := tt.setup(eg)

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
