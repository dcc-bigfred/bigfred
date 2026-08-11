package cmd

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"

	buserrors "github.com/keskad/loco/pkgs/bigfred/dcc-bus/errors"
	"github.com/keskad/loco/pkgs/bigfred/dcc-bus/protocol"
	"github.com/keskad/loco/pkgs/loco/commandstation"
)

type cvCall struct {
	mode   commandstation.Mode
	locoID commandstation.LocoAddr
	cv     uint16
	value  int
}

// cvStubStation records CV traffic and answers reads from a fixed map.
type cvStubStation struct {
	commandstation.StubStation
	mu     sync.Mutex
	values map[uint16]int
	reads  []cvCall
	writes []cvCall
}

func (s *cvStubStation) ReadCV(mode commandstation.Mode, lcv commandstation.LocoCV, _ ...commandstation.Option) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads = append(s.reads, cvCall{mode: mode, locoID: lcv.LocoId, cv: uint16(lcv.Cv.Num)})
	return s.values[uint16(lcv.Cv.Num)], nil
}

func (s *cvStubStation) WriteCV(mode commandstation.Mode, lcv commandstation.LocoCV, _ ...commandstation.Option) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writes = append(s.writes, cvCall{mode: mode, locoID: lcv.LocoId, cv: uint16(lcv.Cv.Num), value: lcv.Cv.Value})
	return nil
}

func newProgrammingRouter(st commandstation.Station, enabled bool, track string) *Router {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return &Router{
		station:                 st,
		log:                     log,
		programmingEnabled:      enabled,
		defaultProgrammingTrack: normalizeProgrammingTrack(track),
	}
}

func TestProgramming_disabledRejectsEveryFrame(t *testing.T) {
	t.Parallel()
	st := &cvStubStation{values: map[uint16]int{}}
	r := newProgrammingRouter(st, false, protocol.ProgrammingModeProg)
	ctx := context.Background()
	actor := Actor{UserID: 1, SessionID: "s1"}

	results := []Result{
		r.HandleLocoCVWrite(ctx, actor, noopResponder{}, protocol.LocoCVWritePayload{CVs: []protocol.CVEntry{{CV: 1, Value: 3}}}, ""),
		r.HandleLocoCVRead(ctx, actor, noopResponder{}, protocol.LocoCVReadPayload{CVs: []uint16{1}}, ""),
		r.HandleLocoAddrSet(ctx, actor, noopResponder{}, protocol.LocoAddrSetPayload{Address: 3}, ""),
		r.HandleLocoAddrGet(ctx, actor, noopResponder{}, protocol.LocoAddrGetPayload{}, ""),
	}
	for i, res := range results {
		if res.OK || res.Code != buserrors.CodeProgrammingDisabled {
			t.Fatalf("result %d = %+v, want %s", i, res, buserrors.CodeProgrammingDisabled)
		}
	}
	if len(st.reads) != 0 || len(st.writes) != 0 {
		t.Fatalf("station touched while programming disabled: reads=%v writes=%v", st.reads, st.writes)
	}
}

func TestProgramming_modeFallsBackToDefaultTrack(t *testing.T) {
	t.Parallel()
	st := &cvStubStation{values: map[uint16]int{8: 151}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModeProg)

	res := r.HandleLocoCVRead(context.Background(), Actor{}, noopResponder{}, protocol.LocoCVReadPayload{
		Address: 42,
		CVs:     []uint16{8},
	}, "")
	if !res.OK {
		t.Fatalf("read failed: %s", res.Code)
	}
	if len(st.reads) != 1 {
		t.Fatalf("reads = %v, want one", st.reads)
	}
	// Service mode addresses the single decoder on the programming
	// track, so the loco id must be dropped even though one was sent.
	if got := st.reads[0]; got.mode != commandstation.ProgrammingTrackMode || got.locoID != 0 {
		t.Fatalf("read call = %+v, want prog mode with loco 0", got)
	}
	if len(res.CVs) != 1 || res.CVs[0].CV != 8 || res.CVs[0].Value != 151 {
		t.Fatalf("cvs = %+v, want CV8=151", res.CVs)
	}
}

func TestProgramming_payloadModeOverridesDefaultTrack(t *testing.T) {
	t.Parallel()
	st := &cvStubStation{values: map[uint16]int{}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModeProg)

	res := r.HandleLocoCVWrite(context.Background(), Actor{}, noopResponder{}, protocol.LocoCVWritePayload{
		Address: 42,
		Mode:    protocol.ProgrammingModePOM,
		CVs:     []protocol.CVEntry{{CV: 3, Value: 7}},
	}, "")
	if !res.OK {
		t.Fatalf("write failed: %s", res.Code)
	}
	if got := st.writes[0]; got.mode != commandstation.MainTrackMode || got.locoID != 42 || got.value != 7 {
		t.Fatalf("write call = %+v, want pom mode addressed to 42", got)
	}
}

func TestProgramming_pomWithoutAddressIsRejected(t *testing.T) {
	t.Parallel()
	st := &cvStubStation{values: map[uint16]int{}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModePOM)

	res := r.HandleLocoCVRead(context.Background(), Actor{}, noopResponder{}, protocol.LocoCVReadPayload{CVs: []uint16{1}}, "")
	if res.OK || res.Code != buserrors.WsCodeBadPayload {
		t.Fatalf("result = %+v, want %s", res, buserrors.WsCodeBadPayload)
	}
}

func TestHandleLocoAddrGet_decodesLongAddress(t *testing.T) {
	t.Parallel()
	// CV29 bit 5 set → address lives in CV17/CV18: (196-192)*256 + 210.
	st := &cvStubStation{values: map[uint16]int{1: 3, 17: 196, 18: 210, 29: 34}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModeProg)

	res := r.HandleLocoAddrGet(context.Background(), Actor{}, noopResponder{}, protocol.LocoAddrGetPayload{}, "")
	if !res.OK {
		t.Fatalf("addrGet failed: %s", res.Code)
	}
	if res.LocoAddress != 1234 || !res.LongAddress {
		t.Fatalf("addr = %d long = %v, want 1234 long", res.LocoAddress, res.LongAddress)
	}
	if len(res.CVs) != 4 {
		t.Fatalf("cvs = %+v, want CV1/17/18/29", res.CVs)
	}
}

func TestHandleLocoAddrSet_preservesOtherCV29Bits(t *testing.T) {
	t.Parallel()
	// CV29 = 6: speed table + 28/128 steps, short address.
	st := &cvStubStation{values: map[uint16]int{29: 6}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModeProg)

	res := r.HandleLocoAddrSet(context.Background(), Actor{}, noopResponder{}, protocol.LocoAddrSetPayload{Address: 1234}, "")
	if !res.OK {
		t.Fatalf("addrSet failed: %s", res.Code)
	}
	if !res.LongAddress || res.LocoAddress != 1234 {
		t.Fatalf("result = %+v, want long 1234", res)
	}
	want := []cvCall{
		{mode: commandstation.ProgrammingTrackMode, cv: 17, value: 196},
		{mode: commandstation.ProgrammingTrackMode, cv: 18, value: 210},
		{mode: commandstation.ProgrammingTrackMode, cv: 29, value: 6 | 32},
	}
	if len(st.writes) != len(want) {
		t.Fatalf("writes = %+v, want %+v", st.writes, want)
	}
	for i, w := range want {
		if st.writes[i] != w {
			t.Fatalf("write %d = %+v, want %+v", i, st.writes[i], w)
		}
	}
}

func TestHandleLocoAddrSet_shortAddressClearsLongBit(t *testing.T) {
	t.Parallel()
	// CV29 = 38: long-address bit set on top of the same config bits.
	st := &cvStubStation{values: map[uint16]int{29: 38}}
	r := newProgrammingRouter(st, true, protocol.ProgrammingModeProg)

	res := r.HandleLocoAddrSet(context.Background(), Actor{}, noopResponder{}, protocol.LocoAddrSetPayload{Address: 7}, "")
	if !res.OK {
		t.Fatalf("addrSet failed: %s", res.Code)
	}
	if res.LongAddress {
		t.Fatalf("address 7 must be programmed as a short address")
	}
	last := st.writes[len(st.writes)-1]
	if last.cv != 29 || last.value != 6 {
		t.Fatalf("CV29 write = %+v, want 6 (long bit cleared)", last)
	}
}
