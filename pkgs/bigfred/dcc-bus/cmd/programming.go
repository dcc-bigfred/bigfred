package cmd

import (
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

const (
	// programmingTimeout bounds one CV read or write round-trip. Service
	// mode acks are slow (a decoder may take several packet periods to
	// answer) so this is far above the throttle-path budget.
	programmingTimeout = 15 * time.Second
	// programmingReadRetries re-issues a read when the decoder stays
	// silent; writes are not retried because they are not idempotent
	// from the decoder's point of view.
	programmingReadRetries = 1
	// programmingSettle lets a decoder finish its internal write cycle
	// before the next CV is pushed (matches the loco CLI default).
	programmingSettle = 300 * time.Millisecond
)

var errNoStation = stderrors.New("dcc-bus: no command station")

// normalizeProgrammingTrack coerces a configured track name onto one of
// the two driver modes. Anything unrecognised falls back to the
// programming track, which cannot disturb locos on the main track.
func normalizeProgrammingTrack(track string) string {
	if track == protocol.ProgrammingModePOM {
		return protocol.ProgrammingModePOM
	}
	return protocol.ProgrammingModeProg
}

// resolveProgrammingTarget maps a frame's `mode` (or the daemon default
// when it is empty) onto the driver's mode / address pair. Service-mode
// programming addresses the single decoder sitting on the programming
// track, so the loco id is always 0 there; POM needs the real address
// because the packet travels over the shared main track.
func (r *Router) resolveProgrammingTarget(mode string, addr uint16) (commandstation.Mode, commandstation.LocoAddr, error) {
	if mode == "" {
		mode = r.defaultProgrammingTrack
	}
	switch mode {
	case protocol.ProgrammingModeProg:
		return commandstation.ProgrammingTrackMode, 0, nil
	case protocol.ProgrammingModePOM:
		if addr == 0 {
			return "", 0, fmt.Errorf("pom programming requires a locomotive address")
		}
		return commandstation.MainTrackMode, commandstation.LocoAddr(addr), nil
	default:
		return "", 0, fmt.Errorf("unsupported programming mode %q", mode)
	}
}

// programmingGate rejects every CV / address use case when the daemon
// was started without --enable-programming, or when no driver is bound.
func (r *Router) programmingGate() (Result, bool) {
	if r == nil || !r.programmingEnabled {
		return FailResult(buserrors.CodeProgrammingDisabled), false
	}
	if r.station == nil {
		return FailResult(buserrors.CodeCommandStationError), false
	}
	return Result{}, true
}

// HandleLocoCVWrite programs the requested CVs on one decoder. Writes
// are applied in the order the client sent them with a settle pause in
// between; the first failure aborts the batch so a half-applied address
// change is reported instead of silently continued.
func (r *Router) HandleLocoCVWrite(_ context.Context, actor Actor, _ Responder, p protocol.LocoCVWritePayload, _ string) Result {
	if res, ok := r.programmingGate(); !ok {
		return res
	}
	mode, locoID, err := r.resolveProgrammingTarget(p.Mode, p.Address)
	if err != nil {
		return r.programmingFailure(actor, "loco.cvWrite", p.Address, err, buserrors.WsCodeBadPayload)
	}

	r.progMu.Lock()
	defer r.progMu.Unlock()

	written := make([]protocol.CVEntry, 0, len(p.CVs))
	for i, entry := range p.CVs {
		if i > 0 {
			time.Sleep(programmingSettle)
		}
		if err := r.writeCV(mode, locoID, entry.CV, int(entry.Value), false); err != nil {
			return r.programmingFailure(actor, "loco.cvWrite", p.Address, err, buserrors.CodeProgrammingFailed)
		}
		written = append(written, entry)
	}

	res := OKResult()
	res.CVs = written
	return res
}

// HandleLocoCVRead reads the requested CVs back from one decoder. A POM
// read needs a RailCom-capable command station; on the programming
// track any decoder answers.
func (r *Router) HandleLocoCVRead(_ context.Context, actor Actor, _ Responder, p protocol.LocoCVReadPayload, _ string) Result {
	if res, ok := r.programmingGate(); !ok {
		return res
	}
	mode, locoID, err := r.resolveProgrammingTarget(p.Mode, p.Address)
	if err != nil {
		return r.programmingFailure(actor, "loco.cvRead", p.Address, err, buserrors.WsCodeBadPayload)
	}

	r.progMu.Lock()
	defer r.progMu.Unlock()

	out := make([]protocol.CVEntry, 0, len(p.CVs))
	for _, num := range p.CVs {
		value, err := r.readCV(mode, locoID, num)
		if err != nil {
			return r.programmingFailure(actor, "loco.cvRead", p.Address, err, buserrors.CodeProgrammingFailed)
		}
		out = append(out, protocol.CVEntry{CV: num, Value: uint8(value)})
	}

	res := OKResult()
	res.CVs = out
	return res
}

// HandleLocoAddrGet decodes the decoder's programmed address from
// CV1 / CV17 / CV18 / CV29 and returns both the raw CVs and the decoded
// address so a UI can show the long/short format it is in.
func (r *Router) HandleLocoAddrGet(_ context.Context, actor Actor, _ Responder, p protocol.LocoAddrGetPayload, _ string) Result {
	if res, ok := r.programmingGate(); !ok {
		return res
	}
	mode, locoID, err := r.resolveProgrammingTarget(p.Mode, p.Address)
	if err != nil {
		return r.programmingFailure(actor, "loco.addrGet", p.Address, err, buserrors.WsCodeBadPayload)
	}

	r.progMu.Lock()
	defer r.progMu.Unlock()

	values := make(map[uint16]int, len(addressCVNums))
	cvs := make([]protocol.CVEntry, 0, len(addressCVNums))
	for _, num := range addressCVNums {
		value, err := r.readCV(mode, locoID, num)
		if err != nil {
			return r.programmingFailure(actor, "loco.addrGet", p.Address, fmt.Errorf("read CV%d: %w", num, err), buserrors.CodeProgrammingFailed)
		}
		values[num] = value
		cvs = append(cvs, protocol.CVEntry{CV: num, Value: uint8(value)})
	}

	addr, long, err := addressFromCVs(values[1], values[17], values[18], values[29])
	if err != nil {
		return r.programmingFailure(actor, "loco.addrGet", p.Address, err, buserrors.CodeProgrammingFailed)
	}

	res := OKResult()
	res.CVs = cvs
	res.LocoAddress = addr
	res.LongAddress = long
	return res
}

// HandleLocoAddrSet rewrites a decoder's DCC address. CV29 is read
// first so only its long-address bit is touched and the operator's
// other decoder settings survive the change.
func (r *Router) HandleLocoAddrSet(_ context.Context, actor Actor, _ Responder, p protocol.LocoAddrSetPayload, _ string) Result {
	if res, ok := r.programmingGate(); !ok {
		return res
	}
	// On the main track a decoder can only be reached at the address it
	// already answers to, so a POM addrSet re-encodes that same address
	// (e.g. short → long format); moving a decoder to a different
	// address needs the programming track.
	mode, locoID, err := r.resolveProgrammingTarget(p.Mode, p.Address)
	if err != nil {
		return r.programmingFailure(actor, "loco.addrSet", p.Address, err, buserrors.WsCodeBadPayload)
	}

	r.progMu.Lock()
	defer r.progMu.Unlock()

	cv29, err := r.readCV(mode, locoID, 29)
	if err != nil {
		return r.programmingFailure(actor, "loco.addrSet", p.Address, fmt.Errorf("read CV29: %w", err), buserrors.CodeProgrammingFailed)
	}

	writes, long, err := addressCVWrites(p.Address, cv29)
	if err != nil {
		return r.programmingFailure(actor, "loco.addrSet", p.Address, err, buserrors.WsCodeBadPayload)
	}

	for i, entry := range writes {
		if i > 0 {
			time.Sleep(programmingSettle)
		}
		if err := r.writeCV(mode, locoID, entry.CV, int(entry.Value), p.Verify); err != nil {
			return r.programmingFailure(actor, "loco.addrSet", p.Address, fmt.Errorf("write CV%d: %w", entry.CV, err), buserrors.CodeProgrammingFailed)
		}
	}

	r.log.WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"userId":    actor.UserID,
		"addr":      p.Address,
		"mode":      mode,
		"long":      long,
	}).Info("dcc-bus decoder address programmed")

	res := OKResult()
	res.CVs = writes
	res.LocoAddress = p.Address
	res.LongAddress = long
	return res
}

func (r *Router) readCV(mode commandstation.Mode, locoID commandstation.LocoAddr, num uint16) (int, error) {
	if r.station == nil {
		return 0, errNoStation
	}
	return r.station.ReadCV(mode, commandstation.LocoCV{
		LocoId: locoID,
		Cv:     commandstation.CV{Num: commandstation.CVNum(num)},
	}, commandstation.Timeout(programmingTimeout), commandstation.Retries(programmingReadRetries))
}

func (r *Router) writeCV(mode commandstation.Mode, locoID commandstation.LocoAddr, num uint16, value int, verify bool) error {
	if r.station == nil {
		return errNoStation
	}
	return r.station.WriteCV(mode, commandstation.LocoCV{
		LocoId: locoID,
		Cv:     commandstation.CV{Num: commandstation.CVNum(num), Value: value},
	}, commandstation.Verify(verify), commandstation.Timeout(programmingTimeout))
}

func (r *Router) programmingFailure(actor Actor, frameType string, addr uint16, err error, code string) Result {
	r.log.WithError(err).WithFields(logrus.Fields{
		"sessionId": actor.SessionID,
		"userId":    actor.UserID,
		"type":      frameType,
		"addr":      addr,
	}).Warn("dcc-bus programming request failed")
	return FailResult(code)
}
