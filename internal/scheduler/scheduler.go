package scheduler

import (
	"context"
	"sync"

	"github.com/Hexa-Ton/administer/internal/classify"
	"github.com/Hexa-Ton/administer/internal/engine"
)

type Reason int

const (
	NoCancel Reason = iota
	ReasonDiscovery
	ReasonUserInterrupt
)

type Scan struct {
	mu           sync.Mutex
	Results      []engine.Result
	Discovery    *engine.Result
	CancelReason Reason
	Completed    int64
	Total        int64
}

// Run drives up to len(engines) concurrent workers against jobs. Each
// job is claimed by exactly one engine, via a single shared channel.
// The scan stops dispatching new jobs, and cancels in-flight
// requests, the moment any result is classified as a Candidate.
func Run(ctx context.Context, engines []*engine.Engine, jobs []string, baseline classify.Baseline, onProgress func(engine.Result)) *Scan {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCh := make(chan string, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan engine.Result, len(jobs)+len(engines))

	var wg sync.WaitGroup
	for _, e := range engines {
		wg.Add(1)
		go func(e *engine.Engine) {
			defer wg.Done()
			for {
				select {
				case <-scanCtx.Done():
					return
				case path, ok := <-jobCh:
					if !ok {
						return
					}
					res := e.Check(scanCtx, path, baseline)
					select {
					case resultCh <- res:
					case <-scanCtx.Done():
					}
				}
			}
		}(e)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	scan := &Scan{Total: int64(len(jobs))}

	for res := range resultCh {
		scan.mu.Lock()
		scan.Results = append(scan.Results, res)
		scan.Completed++
		scan.mu.Unlock()

		if onProgress != nil {
			onProgress(res)
		}

		if res.Verdict == classify.Candidate {
			scan.mu.Lock()
			alreadyFound := scan.Discovery != nil
			if !alreadyFound {
				r := res
				scan.Discovery = &r
				scan.CancelReason = ReasonDiscovery
			}
			scan.mu.Unlock()
			if !alreadyFound {
				cancel()
			}
		}
	}

	if scan.Discovery == nil && ctx.Err() != nil {
		scan.mu.Lock()
		scan.CancelReason = ReasonUserInterrupt
		scan.mu.Unlock()
	}

	return scan
}
