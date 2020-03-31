package discovery_test

import (
	"testing"
	"time"

	"context"

	"github.com/vishen/go-chromecast/discovery"
)

type MockedScanner struct {
	ScanFuncCalled int
	ScanFunc       func(ctx context.Context, results chan<- *discovery.Device) error
}

func (s *MockedScanner) Scan(ctx context.Context, results chan<- *discovery.Device) error {
	s.ScanFuncCalled++
	return s.ScanFunc(ctx, results)
}

func TestFirstDirect(t *testing.T) {
	scan := MockedScanner{
		ScanFunc: func(ctx context.Context, results chan<- *discovery.Device) error {
			go func() {
				results <- &discovery.Device{}
				close(results)
			}()
			return nil
		},
	}

	service := discovery.Service{Scanner: &scan}

	ctx := context.Background()

	first, err := service.First(ctx)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if first == nil {
		t.Errorf("a client should have been found")
	}
	if scan.ScanFuncCalled != 1 {
		t.Errorf("scanner should have been called once, and not %d times", scan.ScanFuncCalled)
	}
}

func TestFirstCancelled(t *testing.T) {
	scan := MockedScanner{
		ScanFunc: func(ctx context.Context, results chan<- *discovery.Device) error {
			go func() {
				<-ctx.Done()
			}()
			return nil
		},
	}

	service := discovery.Service{Scanner: &scan}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	first, err := service.First(ctx)
	if err != ctx.Err() {
		t.Errorf("unexpected error %v", err)
	}
	if first != nil {
		t.Errorf("a client should not have been found")
	}
	if scan.ScanFuncCalled > 1 {
		t.Errorf("scanner should have been called at most once, and not %d times", scan.ScanFuncCalled)
	}
}

func TestNamedDirect(t *testing.T) {
	scan := MockedScanner{}
	done := make(chan struct{})
	scan.ScanFunc = func(ctx context.Context, results chan<- *discovery.Device) error {
		go func() {
			defer close(results)
			results <- &discovery.Device{}
			c := &discovery.Device{
				Properties: map[string]string{
					"fn": "casti",
				},
			}
			results <- c
			results <- &discovery.Device{}
			select {
			case results <- &discovery.Device{}:
				t.Error("channel should have been full")
			case <-ctx.Done():
			}
			close(done)
		}()
		return nil
	}

	service := discovery.Service{Scanner: &scan}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	first, err := service.First(ctx, discovery.WithName("casti"))
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if first == nil {
		t.Fatalf("a client should have been found")
	}
	if first.Name() != "casti" {
		t.Errorf("the client should been named 'casti' and not '%s'", first.Name())
	}
	if scan.ScanFuncCalled != 1 {
		t.Errorf("scanner should have been called once, and not %d times", scan.ScanFuncCalled)
	}
	<-done
}

func TestNamedCancelled(t *testing.T) {
	scan := MockedScanner{}
	done := make(chan struct{})
	scan.ScanFunc = func(ctx context.Context, results chan<- *discovery.Device) error {
		go func() {
			defer close(results)
			for {
				select {
				case results <- &discovery.Device{}:
				case <-ctx.Done():
					close(done)
					return
				}
			}
		}()
		return nil
	}

	service := discovery.Service{Scanner: &scan}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	first, err := service.First(ctx, discovery.WithName("casti"))
	if err != ctx.Err() {
		t.Errorf("unexpected error %v", err)
	}
	if err != ctx.Err() {
		t.Errorf("unexpected error %v", err)
	}
	if first != nil {
		t.Errorf("a client should not have been found")
	}
	if scan.ScanFuncCalled > 1 {
		t.Errorf("scanner should have been called at most once, and not %d times", scan.ScanFuncCalled)
	}
	<-done
}
