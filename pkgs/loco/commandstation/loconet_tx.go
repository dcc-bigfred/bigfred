package commandstation

import (
	"errors"
)

type lnTxPriority bool

const (
	lnTxPriorityNormal lnTxPriority = false
	lnTxPriorityLow    lnTxPriority = true
)

type lnTxJob struct {
	pkt  []byte
	done chan error
}

// txLoop is the sole owner of bus pacing and transport writes. Normal-priority
// traffic (speed, functions, slot ops) always runs before a deferred keepalive
// frame; a low frame taken from txLowCh is held back when normal traffic arrives
// in the same scheduling window.
func (l *LocoNet) txLoop() {
	var pendingLow *lnTxJob
	for {
		if pendingLow != nil {
			select {
			case <-l.stop:
				return
			case nj := <-l.txCh:
				l.txRun(nj)
				continue
			default:
			}
			job := *pendingLow
			pendingLow = nil
			l.txRun(job)
			continue
		}

		select {
		case <-l.stop:
			return
		case job := <-l.txCh:
			l.txRun(job)
		default:
			select {
			case <-l.stop:
				return
			case job := <-l.txCh:
				l.txRun(job)
			case job := <-l.txLowCh:
				select {
				case nj := <-l.txCh:
					deferred := job
					pendingLow = &deferred
					l.txRun(nj)
				default:
					l.txRun(job)
				}
			}
		}
	}
}

func (l *LocoNet) txRun(job lnTxJob) {
	err := l.txWriteOne(job.pkt)
	if job.done != nil {
		job.done <- err
	}
}

// txEnqueue submits one frame to the writer goroutine and blocks until it has
// been paced and written (or the driver is stopping).
func (l *LocoNet) txEnqueue(pkt []byte, low lnTxPriority) error {
	done := make(chan error, 1)
	job := lnTxJob{pkt: pkt, done: done}
	dest := l.txCh
	if low {
		dest = l.txLowCh
	}
	select {
	case dest <- job:
	case <-l.stop:
		return errors.New("loconet: stopped")
	}
	select {
	case err := <-done:
		return err
	case <-l.stop:
		return errors.New("loconet: stopped")
	}
}

// txWriteOne validates, paces, and writes one frame. Only txLoop calls this.
func (l *LocoNet) txWriteOne(pkt []byte) error {
	l.pace(pkt)
	return l.writeRaw(pkt)
}
