// Package ingest owns the asynchronous observation boundary between the BLE
// scanner and the anomaly detector.
package ingest

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/palantir/witchcraft-go-health/v2/conjure/witchcraft/api/health"
	"github.com/palantir/witchcraft-go-health/v2/sources/window"
	"github.com/palantir/witchcraft-go-tasks/executor"
	"github.com/palantir/witchcraft-go-tasks/function"
	"github.com/palantir/witchcraft-go-tasks/workerpool"

	"github.com/niski84/ble-watch/internal/anomaly"
	"github.com/niski84/ble-watch/internal/scanner"
)

// Stats is a snapshot of the observation submission lifecycle.
type Stats struct {
	Submitted uint64 `json:"submitted"`
	Rejected  uint64 `json:"rejected"`
	Processed uint64 `json:"processed"`
}

// Processor submits observations to a single detector worker. Queue storage
// is not bounded. Detector persistence errors are not returned to the submitter.
type Processor struct {
	submitter executor.DelayedItemSubmitter[scanner.Observation]
	submitted atomic.Uint64
	rejected  atomic.Uint64
	processed atomic.Uint64
}

// New creates a single-worker observation processor tied to ctx. Cancelling ctx
// shuts down the queue and causes later submissions to return an error.
func New(ctx context.Context, detector *anomaly.Detector) *Processor {
	p := &Processor{}
	consumer := function.NewConsumerFromFunc(func(ctx context.Context, obs scanner.Observation) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		detector.Process(obs)
		p.processed.Add(1)
		return nil
	})
	workers := workerpool.NewDefaultConsumerWorkerPool(ctx, consumer,
		workerpool.WithMaxNumberOfWorkers(1),
	)
	healthSource := window.MustNewKeyedErrorHealthCheckSource(
		health.CheckType("ble-observation-processing"),
		window.UnhealthyIfAtLeastOneError,
	)
	p.submitter = executor.NewDefaultItemSubmitter(
		ctx,
		workers,
		healthSource,
		executor.WithItemSubmitterName("ble-observations"),
	)
	return p
}

// Submit queues an observation or returns a lifecycle error if the caller or
// the processor is shutting down.
func (p *Processor) Submit(ctx context.Context, obs scanner.Observation) error {
	err := p.submitter.TrySubmit(ctx, obs)
	if err != nil {
		p.rejected.Add(1)
		return err
	}
	p.submitted.Add(1)
	return nil
}

// Stats reads independent atomic counters. Processed counts detector returns,
// not confirmed database commits.
func (p *Processor) Stats() Stats {
	return Stats{
		Submitted: p.submitted.Load(),
		Rejected:  p.rejected.Load(),
		Processed: p.processed.Load(),
	}
}

// IsShutdownError reports whether err means the task queue is no longer
// accepting work.
func IsShutdownError(err error) bool {
	return errors.Is(err, executor.ErrItemSubmitterShutdown) ||
		errors.Is(err, executor.ErrItemSubmitterContext)
}
