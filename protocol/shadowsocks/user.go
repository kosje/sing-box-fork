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

// UpdateUsersByOptions hot-swaps the shadowsocks user set (with passwords) on a
// running MultiInbound without restarting the listener.
func (h *MultiInbound) UpdateUsersByOptions(users []option.ShadowsocksUser) error {
	h.users = users
	return h.service.UpdateUsersWithPasswords(
		common.MapIndexed(h.users, func(index int, user option.ShadowsocksUser) int {
			return index
		}),
		common.Map(h.users, func(user option.ShadowsocksUser) string {
			return user.Password
		}),
	)
}
