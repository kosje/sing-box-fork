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

// UpdateUsers hot-swaps the VLESS user set on a running inbound without
// restarting the listener.
func (h *Inbound) UpdateUsers(users []option.VLESSUser) error {
	h.users = users
	h.service.UpdateUsers(
		common.MapIndexed(h.users, func(index int, _ option.VLESSUser) int {
			return index
		}),
		common.Map(h.users, func(it option.VLESSUser) string {
			return it.UUID
		}),
		common.Map(h.users, func(it option.VLESSUser) string {
			return it.Flow
		}),
	)
	return nil
}
