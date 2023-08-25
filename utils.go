package main

import "runtime"

func sliceToMap[K comparable, V any](slice []V, convert func(V) K) map[K]V {
	res := make(map[K]V, len(slice))
	for _, v := range slice {
		res[convert(v)] = v
	}
	return res
}

// func All[A any, R any](fn func(a A) (R, error), args ...A) ([]R, error) {
// 	type Res struct {
// 		i   int
// 		val R
// 		err error
// 	}
// 	ch := make(chan *Res, len(args))
// 	for i, arg := range args {
// 		go func(i int, arg A) {
// 			v, err := fn(arg)
// 			ch <- &Res{i, v, err}
// 		}(i, arg)
// 	}
// 	arr := make([]R, len(args))
// 	for range args {
// 		res := <-ch
// 		if res.err != nil {
// 			return arr, res.err
// 		}
// 		arr[res.i] = res.val
// 	}
// 	return arr, nil
// }

// func AllSettled[A any, R any](fn func(a A) (R, error), args ...A) ([]R, []error) {
// 	type Res struct {
// 		i   int
// 		val R
// 		err error
// 	}
// 	ch := make(chan *Res, len(args))
// 	for i, arg := range args {
// 		go func(i int, arg A) {
// 			v, err := fn(arg)
// 			ch <- &Res{i, v, err}
// 		}(i, arg)
// 	}
// 	arr := make([]R, len(args))
// 	errs := make([]error, len(args))
// 	for range args {
// 		res := <-ch
// 		errs[res.i] = res.err
// 		arr[res.i] = res.val
// 	}
// 	return arr, errs
// }

func maxParallelism() int {
	maxProcs := runtime.GOMAXPROCS(0)
	numCPU := runtime.NumCPU()
	if maxProcs < numCPU {
		return maxProcs
	}
	return numCPU
}
