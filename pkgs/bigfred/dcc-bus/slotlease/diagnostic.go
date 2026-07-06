package slotlease

import (
	"sort"
	"time"
)

// HolderInfo is one driver session on a leased address (D19).
type HolderInfo struct {
	UserID      uint   `json:"userId"`
	Session     string `json:"session"`
	Source      string `json:"source"`
	LastDriveAt int64  `json:"lastDriveAt"` // unix ms; 0 for WS holders
}

// LeaseInfo is one active slot lease (D19).
type LeaseInfo struct {
	Addr           uint16       `json:"addr"`
	Kind           string       `json:"kind"` // "single" | "train"
	TrainID        string       `json:"trainId,omitempty"`
	Holders        []HolderInfo `json:"holders"`
	AcquiredAt     int64        `json:"acquiredAt"` // unix ms
	ReleasePending bool         `json:"releasePending"`
}

// SlotsDiagnostic is a point-in-time leaser snapshot for admin UI (D19).
type SlotsDiagnostic struct {
	MaxPerUser int            `json:"maxPerUser"`
	MaxSlots   int            `json:"maxSlots"`
	Used       int            `json:"used"`
	PerUser    map[uint]int   `json:"perUser"`
	Leases     []LeaseInfo    `json:"leases"`
	At         int64          `json:"at"` // unix ms
}

// DiagEvents exposes lease mutation notifications for the admin WS stream.
func (l *Leaser) DiagEvents() <-chan struct{} {
	if l == nil {
		return nil
	}
	return l.diagCh
}

// DiagnosticSnapshot returns the current lease table under leaser mu.
func (l *Leaser) DiagnosticSnapshot() SlotsDiagnostic {
	if l == nil {
		return SlotsDiagnostic{At: time.Now().UnixMilli()}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snapshotLocked(time.Now())
}

func (l *Leaser) snapshotLocked(now time.Time) SlotsDiagnostic {
	out := SlotsDiagnostic{
		MaxPerUser: l.maxPerUser,
		MaxSlots:   l.maxSlots,
		Used:       l.budgetActiveLocked(),
		PerUser:    make(map[uint]int, len(l.perUser)),
		At:         now.UnixMilli(),
	}
	for uid, n := range l.perUser {
		out.PerUser[uid] = n
	}
	out.Leases = make([]LeaseInfo, 0, len(l.leases))
	for _, le := range l.leases {
		_, pending := l.releasePending[le.addr]
		info := LeaseInfo{
			Addr: le.addr,
			// Always a non-nil slice so JSON encodes [] not null (grace leases
			// have zero holders while releaseAt is set).
			Holders: make([]HolderInfo, 0, len(le.holders)),
			// Grace window (switcher) or driver release retry both surface as pending.
			ReleasePending: pending || !le.releaseAt.IsZero(),
		}
		if le.kind == leaseTrain {
			info.Kind = "train"
			info.TrainID = le.trainID
		} else {
			info.Kind = "single"
		}
		if !le.acquiredAt.IsZero() {
			info.AcquiredAt = le.acquiredAt.UnixMilli()
		}
		for _, k := range le.holderOrder {
			if _, ok := le.holders[k]; !ok {
				continue
			}
			hi := HolderInfo{
				UserID:  k.UserID,
				Session: shortenSession(k.Session),
				Source:  k.Source,
			}
			if at, ok := le.lastDriveAt[k]; ok && k.Source != "ws" {
				hi.LastDriveAt = at.UnixMilli()
			}
			info.Holders = append(info.Holders, hi)
		}
		out.Leases = append(out.Leases, info)
	}
	sort.Slice(out.Leases, func(i, j int) bool { return out.Leases[i].Addr < out.Leases[j].Addr })
	return out
}

func (l *Leaser) gaugeSnapshot() (used int, activeBySource map[string]int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	used = l.budgetActiveLocked()
	activeBySource = make(map[string]int, 3)
	for _, le := range l.leases {
		seen := make(map[string]struct{}, 2)
		for k := range le.holders {
			seen[k.Source] = struct{}{}
		}
		for src := range seen {
			if src == "" {
				src = "_"
			}
			activeBySource[src]++
		}
	}
	return used, activeBySource
}

func (l *Leaser) notifyDiagLocked() {
	l.notifyDiag()
}

func (l *Leaser) notifyDiag() {
	if l == nil || l.diagCh == nil {
		return
	}
	select {
	case l.diagCh <- struct{}{}:
	default:
	}
}

func shortenSession(session string) string {
	if len(session) <= 8 {
		return session
	}
	return session[:8]
}
