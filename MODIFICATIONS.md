# Modifications to sing-box

This is a fork of [sing-box](https://github.com/SagerNet/sing-box) **v1.14.0**,
licensed GPL-3.0-or-later. It is **not** affiliated with or endorsed by the
sing-box project.

The fork exists for one reason: sing-box builds its inbound user sets at
construction time only, so a panel that adds or removes users has to restart the
listener and drop every live connection. The underlying `sing-*` services all
already have an `UpdateUsers` method — upstream simply never exposes it on the
`Inbound` types, and [declined to add a user-management API](https://github.com/SagerNet/sing-box/issues/2671).
These patches expose what is already there.

## The complete diff against upstream v1.14.0

Six files, ~110 lines. Nothing else is touched.

| File | Change |
|---|---|
| `adapter/inbound.go` | +15 — declares `UpdatableInbound[T]` and `UpdatableShadowsocksInbound` |
| `protocol/vless/user.go` | new — `(*Inbound).UpdateUsers([]option.VLESSUser)` |
| `protocol/shadowsocks/user.go` | new — `(*MultiInbound).UpdateUsersByOptions([]option.ShadowsocksUser)` |
| `protocol/anytls/user.go` | new — `(*Inbound).UpdateUsers([]option.AnyTLSUser)` |
| `experimental/v2rayapi/stats.go` | ~+35 — `(*StatsService).UpdateUsers([]string)`, so hot-added users are billed |
| `experimental/clashapi/connections.go` | +1 — adds `"user"` to the connection JSON so callers can attribute a connection to a user |

### Why the stats patch is not optional

`StatsService` builds its `users` set once, in `NewStatsService`, and every
routed connection checks `user != "" && s.users[user]` to decide whether to
count traffic. Swapping an inbound's user set at runtime therefore gets you a
user who can connect and relay traffic that is **never counted** — silently, with
no error anywhere, until the next full config apply.

That is worse than the restart it was meant to avoid: a panel bills from these
counters, so the user rides for free and nothing looks wrong.

The patch turns `users` into an `atomic.Pointer[map[string]bool]` and adds
`UpdateUsers`, which swaps the whole map. The read path stays lock-free —
it runs on every routed connection, so taking a mutex there would put lock
contention directly in the forwarding path. Existing counters are left alone:
a removed user keeps what it accumulated until the panel's next `reset=true`
poll, which is the behaviour a billing poller expects.

Each `user.go` is a thin wrapper over the service call the constructor already
makes, plus a compile-time interface assertion:

```go
var _ adapter.UpdatableInbound[option.AnyTLSUser] = (*Inbound)(nil)
```

That assertion matters. The consumer finds these inbounds with a runtime type
assertion, so without it a signature drift would silently turn user changes into
a no-op — the logs would still say "hot-swapped" while nothing happened.

## Adding another protocol

Upstream's trojan, vmess, tuic, hysteria and hysteria2 inbounds all call
`service.UpdateUsers(...)` in their constructors too. To make one hot-swappable,
copy `protocol/anytls/user.go`, match the option type, and add the matching
`Update*Users` method on the engine side. No engine or adapter changes needed —
`UpdatableInbound[T]` is generic.

## Rebasing onto a newer sing-box

1. Extract the new upstream release.
2. Re-apply the five changes above.
3. Check that the fields each wrapper touches still exist: `vless.Inbound.users`
   / `.service`, `shadowsocks.MultiInbound.users` / `.service`,
   `anytls.Inbound.service`.
4. Build. The compile-time assertions fail loudly if an interface drifted.

Known moves so far: in 1.14.0 the clash connection JSON moved from
`experimental/clashapi/trafficontrol/tracker.go` to
`experimental/clashapi/connections.go`, and the receiver changed from
`t TrackerMetadata` to `c`.
