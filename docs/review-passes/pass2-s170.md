# Codex review pass 2 — S170 instance v2alpha1 private-network-interfaces

Four findings, **all accepted**. Three were zone bugs in the handlers I had just
written; the fourth turned out to be load-bearing rather than speculative.

## [P2] ×3 — the routes are zone-scoped and the lookups were not

`CreatePrivateNetworkInterfaceV2` stored the URL's zone without checking the
server was in it; `Get` and `Delete` looked up by id alone. So a client with the
wrong zone could read, update, or **delete** another zone's interface — and
against a mock that wrong client *passes*, which is the failure a fidelity mock
exists to prevent.

The codebase already knew this mattered: `ListPrivateNetworkInterfacesV2` filters
by zone with a comment saying the server_id lookup is not zonal. I added three
routes beside it and did not apply the same rule.

Fixed via one `privateNetworkInterfaceInZone` helper, checked **before** the
delete — the one mistake here that retrying cannot undo. Pinned by
`TestContract_instance_v2_pni_zone_scoped`, covering get, delete and
cross-zone create.

## [P2] Register the PATCH route — checked before believing

This one looked like scope creep: the slice's job was create/get/delete so a NIC
can be applied and destroyed, and a PATCH nothing calls is dead code. YAGNI says
decline.

So it was tested rather than argued. `tofu plan` on a tags change reports:

```
# scaleway_instance_private_nic.web will be updated in-place
Plan: 0 to add, 1 to change, 0 to destroy.
```

and the apply then failed with `501 Not Implemented: PATCH
/instance/v2alpha1/zones/fr-par-1/private-network-interfaces/{id}`.

**So a missing PATCH is not a coverage gap, it is a broken apply** — the same
class of failure this whole slice exists to remove, one field along. Accepted and
implemented, with `UpdatePrivateNIC` on the repository and the unwrapped response
the SDK expects. Verified end to end: a tag change now applies in place.

## Nothing declined this pass.

Worth recording: the instinct to decline finding 4 was reasonable and would have
been wrong. The thing that settled it was a two-minute experiment, not a longer
argument — which is the same lesson the NIC blocker itself taught an hour
earlier.
