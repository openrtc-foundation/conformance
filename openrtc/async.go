package openrtc

import (
	"context"
	"fmt"
	"sync/atomic"
)

type operationObserver interface {
	OperationStarted(OperationID)
	OperationSettled(OperationID, any, error)
}
type asyncResult[T any] struct {
	id    OperationID
	done  chan struct{}
	value T
	err   error
}

var nextOperation atomic.Uint64

// StartOperation allocates correlation before invoking work. Result may be read
// repeatedly; cancelling a waiter does not settle or restart the operation.
func StartOperation[T any](owner any, work func() (T, error)) Async[T] {
	a := &asyncResult[T]{id: OperationID(fmt.Sprintf("operation-%d", nextOperation.Add(1))), done: make(chan struct{})}
	observer, _ := owner.(operationObserver)
	if observer != nil {
		observer.OperationStarted(a.id)
	}
	go func() {
		defer close(a.done)
		a.value, a.err = work()
		if observer != nil {
			observer.OperationSettled(a.id, a.value, a.err)
		}
	}()
	return a
}
func (a *asyncResult[T]) OperationID() OperationID { return a.id }
func (a *asyncResult[T]) Result(ctx context.Context) (T, error) {
	select {
	case <-a.done:
		return a.value, a.err
	default:
	}
	select {
	case <-a.done:
		return a.value, a.err
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

type discardedResult[T any] struct{ Async[T] }

// Discard preserves correlation and settlement while projecting a void public result.
func Discard[T any](operation Async[T]) Async[Void] { return discardedResult[T]{operation} }
func (a discardedResult[T]) Result(ctx context.Context) (Void, error) {
	_, err := a.Async.Result(ctx)
	return Void{}, err
}
