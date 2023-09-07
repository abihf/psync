package main

import "context"

func worker(ctx context.Context, ch chan Task) {
	for {
		select {
		case <-ctx.Done():
			return

		case t := <-ch:
			if t == nil {
				return
			}
			err := doWork(context.Background(), t)
			t.Done(err)
			if err != nil {
				return
			}
		}
	}

}

func doWork(ctx context.Context, task Task) error {
	unlock := task.Wait()
	defer unlock()

	t := task
	for t != nil {
		err := t.Run()
		if err != nil {
			return err
		}
		t = t.Sub()
	}

	return nil
}
