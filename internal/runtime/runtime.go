package runtime

import (
	"context"
	"sync"
)

// RunWorkers starts n goroutines and returns the first error, if any.
func RunWorkers(ctx context.Context, n int, fn func(context.Context) error) error {
	if n < 1 {
		n = 1
	}
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(ctx); err != nil {
				errCh <- err
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
