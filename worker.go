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

			if !t.Do() {
				return
			}
		}
	}

}
