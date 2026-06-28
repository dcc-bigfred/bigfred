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

// txLoop is the sole owner of bus pacing and transport writes. It always
// drains normal-priority traffic (speed, functions, slot ops) before
// keepalive frames so a periodic refresh burst cannot delay throttle commands.
func (l *LocoNet) txLoop() {
	for {
		var job lnTxJob
		select {
		case <-l.stop:
			return
		case job = <-l.txCh:
		default:
			select {
			case <-l.stop:
				return
			case job = <-l.txCh:
			case job = <-l.txLowCh:
			}
		}
		err := l.txWriteOne(job.pkt)
		if job.done != nil {
			job.done <- err
		}
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
