# skysbx-core

A fork of [sing-box](https://github.com/SagerNet/sing-box) **v1.14.0**, carrying
eight patched files (~180 lines) so that inbound user sets can be changed
without restarting the listener. Everything else is upstream, untouched.

**This project is not affiliated with or endorsed by the sing-box project.
Do not report issues with this fork to them.**

It exists to be compiled into [`skysbx-node`](https://github.com/kosje/skysbx-node),
which embeds it rather than running it as a separate process. There is no
separate core version to manage: rebuilding the node is what upgrades it.

The complete diff against upstream, why each patch is there, and how to rebase
it onto a newer sing-box, are in [`MODIFICATIONS.md`](MODIFICATIONS.md).

Upstream's own README follows.

---

# sing-box

The universal proxy platform.

[![Packaging status](https://repology.org/badge/vertical-allrepos/sing-box.svg)](https://repology.org/project/sing-box/versions)

## Documentation

https://sing-box.sagernet.org

## License

```
Copyright (C) 2022 by nekohasekai <contact-sagernet@sekai.icu>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.

In addition, no derivative work may use the name or imply association
with this application without prior consent.
```
