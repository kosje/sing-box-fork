package anytls

import (
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"

	anytls "github.com/anytls/sing-anytls"
)

// Compile-time guarantee that the engine's UpdatableInbound type assertion
// succeeds. Without this, a signature drift would silently downgrade AnyTLS
// user changes to a no-op instead of failing the build.
var _ adapter.UpdatableInbound[option.AnyTLSUser] = (*Inbound)(nil)

// UpdateUsers hot-swaps the AnyTLS user set on a running inbound without
// restarting the listener. The conversion mirrors NewInbound so a hot-swapped
// user set is byte-for-byte what a fresh inbound would have built.
func (h *Inbound) UpdateUsers(users []option.AnyTLSUser) error {
	h.service.UpdateUsers(common.Map(users, func(it option.AnyTLSUser) anytls.User {
		return anytls.User(it)
	}))
	return nil
}
