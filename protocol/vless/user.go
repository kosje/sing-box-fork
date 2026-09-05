package vless

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

// Compile-time guarantee that the engine's UpdatableInbound type assertion
// succeeds. Without this, a signature drift would silently downgrade VLESS user
// changes to a no-op instead of failing the build.
var _ adapter.UpdatableInbound[option.VLESSUser] = (*Inbound)(nil)

// storeUserNames publishes the index -> name mapping that the connection path
// reads. The connection path only ever needs the name, so we snapshot just the
// names rather than the whole user list: it keeps the atomic value small and
// makes it obvious that nothing else may be read from it concurrently.
func (h *Inbound) storeUserNames(users []option.VLESSUser) {
	names := common.Map(users, func(it option.VLESSUser) string {
		return it.Name
	})
	h.userNames.Store(&names)
}

// userName resolves the index that sing-vmess put in the connection context
// back to a panel user name.
//
// The index is minted by h.service, which is updated separately from the name
// snapshot, so during a hot-swap the two can disagree by a few entries for a
// few microseconds. Returning "" for an out-of-range index makes the caller
// fall back to the numeric index, which is the same thing it already does for
// an unnamed user. Indexing h.users directly here — as upstream does — panics
// instead, taking the whole node down on an ordinary user removal.
func (h *Inbound) userName(index int) string {
	names := h.userNames.Load()
	if names == nil || index < 0 || index >= len(*names) {
		return ""
	}
	return (*names)[index]
}

// UpdateUsers hot-swaps the VLESS user set on a running inbound without
// restarting the listener.
//
// The name snapshot and the service's index space are published in the order
// that keeps the snapshot a superset of the indices the service can hand out,
// so a concurrent connection never falls back to a numeric name:
//
//	growing   publish names first  — the service cannot yet mint the new indices
//	shrinking publish names last   — the service has already stopped minting them
//
// Correctness does not depend on this; userName is bounds-checked either way.
// This only avoids mislabelling (and therefore not billing) a connection that
// happens to arrive mid-swap.
func (h *Inbound) UpdateUsers(users []option.VLESSUser) error {
	grew := len(users) >= len(h.users)
	h.users = users
	if grew {
		h.storeUserNames(users)
	}
	h.service.UpdateUsers(
		common.MapIndexed(users, func(index int, _ option.VLESSUser) int {
			return index
		}),
		common.Map(users, func(it option.VLESSUser) string {
			return it.UUID
		}),
		common.Map(users, func(it option.VLESSUser) string {
			return it.Flow
		}),
	)
	if !grew {
		h.storeUserNames(users)
	}
	return nil
}
