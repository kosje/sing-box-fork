package shadowsocks

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
)

// Compile-time guarantee that the engine's UpdatableShadowsocksInbound type
// assertion succeeds. Without this, a signature drift would silently downgrade
// Shadowsocks user changes to a no-op instead of failing the build.
var _ adapter.UpdatableShadowsocksInbound = (*MultiInbound)(nil)

// storeUserNames publishes the index -> name mapping that the connection path
// reads. Only the names are snapshotted: the passwords live in the service and
// must not be reachable from the forwarding path.
func (h *MultiInbound) storeUserNames(users []option.ShadowsocksUser) {
	names := common.Map(users, func(user option.ShadowsocksUser) string {
		return user.Name
	})
	h.userNames.Store(&names)
}

// userName resolves the index that sing-shadowsocks put in the connection
// context back to a panel user name.
//
// The index is minted by h.service, which is updated separately from the name
// snapshot, so during a user change the two can disagree by a few entries for a
// few microseconds. Returning "" for an out-of-range index makes the caller
// fall back to the numeric index, which is what it already does for an unnamed
// user. Indexing h.users directly — as upstream does — panics instead, taking
// the whole node down on an ordinary user change.
func (h *MultiInbound) userName(index int) string {
	names := h.userNames.Load()
	if names == nil || index < 0 || index >= len(*names) {
		return ""
	}
	return (*names)[index]
}

// UpdateUsersByOptions hot-swaps the shadowsocks user set (with passwords) on a
// running MultiInbound without restarting the listener.
//
// The name snapshot and the service's index space are published in the order
// that keeps the snapshot a superset of the indices the service can hand out,
// so a connection arriving mid-swap is still attributed to the right user:
//
//	growing   publish names first  — the service cannot yet mint the new indices
//	shrinking publish names last   — the service has already stopped minting them
//
// Correctness does not depend on this; userName is bounds-checked either way.
func (h *MultiInbound) UpdateUsersByOptions(users []option.ShadowsocksUser) error {
	grew := len(users) >= len(h.users)
	h.users = users
	if grew {
		h.storeUserNames(users)
	}
	err := h.service.UpdateUsersWithPasswords(
		common.MapIndexed(users, func(index int, user option.ShadowsocksUser) int {
			return index
		}),
		common.Map(users, func(user option.ShadowsocksUser) string {
			return user.Password
		}),
	)
	if !grew {
		h.storeUserNames(users)
	}
	return err
}
