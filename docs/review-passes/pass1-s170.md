# Codex review pass 1 — S170 instance v2alpha1 private-network-interfaces

`codex exec review --base main` on `s170-instance-v2-private-network-interfaces`.

One finding, accepted.

## [P2] The missing-server check used the wrong error helper

`CreatePrivateNetworkInterfaceV2` verified the server exists and reported a
failure through `writeDomainError`, which says *"resource not found"* — the
spelling for a resource you asked for directly.

On this path the server is being **referenced**, and the private-network foreign
key inside `CreatePrivateNIC` already answers that way, via `writeCreateError`.
So the *same missing server* produced two different bodies depending on which
check happened to catch it first: `{"message":"resource not found"}` from the
explicit check, `{"message":"referenced resource not found"}` from the FK. A
client cannot code against that.

Fixed with `writeCreateErrorFor(w, err, "server", serverID)`, which also carries
`resource` and `resource_id`, and the contract test now pins the body rather
than only the status.

## Re-reading the fix against its own class

Class: *two paths reporting one condition differently*. The other two new
handlers were checked — `GetPrivateNetworkInterfaceV2` and
`DeletePrivateNetworkInterfaceV2` both use `writeDomainError`, and correctly:
there the interface is the thing being asked for, not a reference.

## Nothing declined this pass.
