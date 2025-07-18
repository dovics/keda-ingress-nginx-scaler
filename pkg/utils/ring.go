package utils

type Ring[T any] struct {
	data  []T
	index int
	size  int
	count int
}

func NewRing[T any](size int) *Ring[T] {
	return &Ring[T]{
		size: size,
		data: make([]T, size),
	}
}

func (r *Ring[T]) Enqueue(item T) {
	r.data[r.index] = item
	r.count++

	r.index = (r.index + 1) % len(r.data)
}

func (r *Ring[T]) Count() int {
	return r.count
}
func (r *Ring[T]) Get(index int) T {
	return r.data[index]
}

func (r *Ring[T]) GetLatest() T {
	return r.data[r.index-1]
}

func (r *Ring[T]) GetBefore(before int) T {
	return r.data[(r.size+r.index-1-before)%r.size]
}
