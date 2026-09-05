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

Eight files, ~180 lines. Nothing else is touched.

| File | Change |
|---|---|
| `adapter/inbound.go` | +15 — declares `UpdatableInbound[T]` and `UpdatableShadowsocksInbound` |
| `protocol/vless/user.go` | new — `(*Inbound).UpdateUsers([]option.VLESSUser)`, plus the name snapshot |
| `protocol/vless/inbound.go` | +4 — `userNames` field; the two connection-path reads go through `userName()` |
| `protocol/shadowsocks/user.go` | new — `(*MultiInbound).UpdateUsersByOptions([]option.ShadowsocksUser)`, plus the name snapshot |
| `protocol/shadowsocks/inbound_multi.go` | +6 — same, and upstream's own `UpdateUsers` keeps the snapshot in step |
| `protocol/anytls/user.go` | new — `(*Inbound).UpdateUsers([]option.AnyTLSUser)` |
| `experimental/v2rayapi/stats.go` | ~+35 — `(*StatsService).UpdateUsers([]string)`, so hot-added users are billed |
| `experimental/clashapi/connections.go` | +1 — adds `"user"` to the connection JSON so callers can attribute a connection to a user |

### Why the index-to-name lookup had to change

VLESS and Shadowsocks identify a user by an **index** into the inbound's user
slice: `sing-vmess` and `sing-shadowsocks` put that index in the connection
context, and the inbound reads `h.users[userIndex].Name` on every accepted
connection to label and bill it.

The index is minted by the *service*, which is updated separately from
`h.users`. Swapping users therefore leaves a window where the two disagree, and
whichever is shorter loses:

```
removing a user   h.users shrinks to 3, the service still hands out index 4
                  -> h.users[4] -> index out of range -> the node panics
```

An ordinary user removal — an expiry, a disable — could take the whole data
plane down under load. Upstream's own `ManagedSSMServer` path has the same
defect in the other direction: it updates the service first, so *adding* users
is the dangerous case there.

The fix is a `userNames atomic.Pointer[[]string]` snapshot published alongside
the service update, and a bounds-checked `userName(index)` in place of the raw
slice index. An out-of-range index now returns `""`, and the caller already
knows how to fall back to the numeric index for an unnamed user.

Publication order narrows the window further — names first when growing, last
when shrinking, so the snapshot is always a superset of the indices the service
can mint. Correctness does not rest on that; it only avoids mislabelling (and
so not billing) a connection that lands mid-swap.

AnyTLS needs none of this: it carries the user *name* in the connection context
(`auth.UserFromContext[string]`), not an index.

Both the defect and the fix were confirmed with the race detector, running the
node against a live panel while 600 connections were opened and 20 users were
created and deleted (120 hot-swaps):

```
before   WARNING: DATA RACE
           Write  protocol/shadowsocks/user.go:17   h.users = users
           Read   protocol/shadowsocks/inbound_multi.go:166  h.users[userIndex].Name
         8 of 11 reports named these sites

after    0 reports name any site in this repository
```

Building with `-race` also requires `-gcflags=all=-d=checkptr=0`: sing-box's own
unsafe pointer arithmetic trips `checkptr`, and the node dies at startup before
anything can be observed.

### What is *not* fixed: the same race one layer down

The race detector also reports `sing-vmess`'s `vless.Service` and
`sing-shadowsocks`'s `shadowaead_2022.MultiService` racing with themselves:
each publishes its user maps in `UpdateUsers` with plain unguarded assignments
while `NewConnection` reads them.

These are separate modules, so the patches here cannot reach them. They stay
reported rather than fixed, because the practical impact is small:

- both **replace** their maps wholesale rather than mutating one in place, so
  this cannot produce Go's unrecoverable `concurrent map read and map write`
- each store is a single word, so on amd64/arm64 a reader observes either the
  old map or the new one, never a torn value

The realistic worst case is a connection arriving mid-swap that reads a stale
map — a just-removed user gets one more connection through, or a just-added one
is rejected once — plus, in the Shadowsocks case, a reader that sees the new
`uPSKHash` next to the old `uPSK` and fails that one handshake. All of it
self-corrects on the next connection.

That is categorically milder than what the `h.users` patch fixed, where the
read was a **three-word slice header plus an index**: a length mismatch alone
(no tearing required) sends `h.users[4]` into a three-element slice and takes
the process down.

Upstream never hits any of this because it only calls `UpdateUsers` from a
constructor, single-threaded. Exposing it at runtime — the entire point of this
fork — is what turns it into a live code path.

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
4. Re-check the connection paths for new `h.users[...]` reads. Upstream keeps
   indexing the slice directly; every such site has to go through `userName()`
   or the panic described above comes back. Currently two sites in
   `protocol/vless/inbound.go` and two in `protocol/shadowsocks/inbound_multi.go`:

       grep -n 'h\.users\[' protocol/vless/inbound.go protocol/shadowsocks/inbound_multi.go

   should print nothing after a rebase.
5. Build. The compile-time assertions fail loudly if an interface drifted.

Known moves so far: in 1.14.0 the clash connection JSON moved from
`experimental/clashapi/trafficontrol/tracker.go` to
`experimental/clashapi/connections.go`, and the receiver changed from
`t TrackerMetadata` to `c`.
