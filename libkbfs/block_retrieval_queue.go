package libkbfs

import (
	"container/heap"
	"sync"

	"golang.org/x/net/context"
)

const (
	defaultBlockRetrievalWorkerQueueSize int = 100
)

type blockRetrievalRequest struct {
	ctx    context.Context
	block  Block
	doneCh chan error
}

type blockRetrieval struct {
	blockPtr       BlockPointer
	index          int
	priority       int
	insertionOrder uint64
	requests       []*blockRetrievalRequest
}

type blockRetrievalQueue struct {
	mtx sync.RWMutex
	// queued or in progress retrievals
	ptrs map[BlockPointer]*blockRetrieval
	// global counter of insertions to queue
	// capacity: ~584 years at 1 billion requests/sec
	insertionCount uint64

	heap        *blockRetrievalHeap
	workerQueue chan chan *blockRetrieval
}

func newBlockRetrievalQueue(numWorkers int) *blockRetrievalQueue {
	return &blockRetrievalQueue{
		ptrs:        make(map[BlockPointer]*blockRetrieval),
		heap:        &blockRetrievalHeap{},
		workerQueue: make(chan chan *blockRetrieval, numWorkers),
	}
}

func (brq *blockRetrievalQueue) notifyWorker() {
	go func() {
		// Get the next queued worker
		ch := <-brq.workerQueue
		// Prevent interference with the heap while we're retrieving from it
		brq.mtx.Lock()
		defer brq.mtx.Unlock()
		// Pop from the heap
		ch <- heap.Pop(brq.heap).(*blockRetrieval)
	}()
}

func (brq *blockRetrievalQueue) Request(ctx context.Context, priority int, ptr BlockPointer, block Block) <-chan error {
	brq.mtx.Lock()
	defer brq.mtx.Unlock()
	var br *blockRetrieval
	var exists bool
	if br, exists = brq.ptrs[ptr]; !exists {
		// Add to the heap
		br = &blockRetrieval{
			blockPtr:       ptr,
			index:          -1,
			priority:       priority,
			insertionOrder: brq.insertionCount,
			requests:       []*blockRetrievalRequest{},
		}
		brq.insertionCount++
		brq.ptrs[ptr] = br
		heap.Push(brq.heap, br)
		defer brq.notifyWorker()
	}
	ch := make(chan error, 1)
	br.requests = append(br.requests, &blockRetrievalRequest{ctx, block, ch})
	// If the new request priority is higher, elevate the request in the queue
	if priority > br.priority {
		br.priority = priority
		heap.Fix(brq.heap, br.index)
	}
	return ch
}

func (brq *blockRetrievalQueue) WorkOnRequest() <-chan *blockRetrieval {
	ch := make(chan *blockRetrieval, 1)
	brq.workerQueue <- ch

	return ch
}

// FinalizeRequest communicates that any subsequent requestors for this block
// won't be notified by the current worker processing it.  This must be called
// before sending out the responses to the blockRetrievalRequests for a given
// blockRetrieval.
func (brq *blockRetrievalQueue) FinalizeRequest(ptr BlockPointer) {
	brq.mtx.Lock()
	defer brq.mtx.Unlock()

	delete(brq.ptrs, ptr)
}
