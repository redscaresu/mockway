#!/usr/bin/env python3
"""
spec_diff.py — cross-reference mockway registered routes against Scaleway OpenAPI specs.

Usage:
    python3 scripts/spec_diff.py [--all]

By default prints only HIGH-priority gaps (those the Terraform provider calls
during normal apply/destroy). Pass --all to see every unimplemented spec operation.

Requires: PyYAML  (pip install pyyaml)
"""

import yaml, glob, re, sys, os

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HANDLERS_GO = os.path.join(REPO_ROOT, "handlers", "handlers.go")
SPECS_DIR   = os.path.join(REPO_ROOT, "specs")

IN_SCOPE_PREFIXES = [
    "/instance/v1/zones/",
    "/vpc/v2/regions/",
    "/lb/v1/zones/",
    "/k8s/v1/regions/",
    "/rdb/v1/regions/",
    "/redis/v1/zones/",
    "/registry/v1/regions/",
    "/iam/v1alpha1/",
    "/marketplace/v2/",
]

# Operations the Terraform provider calls during normal apply/destroy.
# Used to flag high-priority gaps.
HIGH_PRIORITY_OPS = {
    ("PATCH", "/instance/v1/zones/{x}/servers/{x}"),
    ("PATCH", "/instance/v1/zones/{x}/ips/{x}"),
    ("GET",   "/instance/v1/zones/{x}/servers/{x}/user_data/{x}"),
    ("PATCH", "/vpc/v2/regions/{x}/vpcs/{x}"),
    ("PATCH", "/vpc/v2/regions/{x}/private-networks/{x}"),
    ("PATCH", "/lb/v1/zones/{x}/ips/{x}"),
    ("PATCH", "/iam/v1alpha1/api-keys/{x}"),
    ("PATCH", "/iam/v1alpha1/applications/{x}"),
    ("PATCH", "/iam/v1alpha1/policies/{x}"),
    ("PATCH", "/iam/v1alpha1/ssh-keys/{x}"),
    ("PUT",   "/iam/v1alpha1/rules"),
    ("GET",   "/k8s/v1/regions/{x}/nodes/{x}"),
}


def norm(path):
    return re.sub(r'\{[^}]+\}', '{x}', path)


def load_registered_routes(handlers_go_path):
    registered = set()
    prefix_stack = []
    with open(handlers_go_path) as f:
        for line in f:
            s = line.strip()
            m = re.search(r'r\.Route\("([^"]+)"', s)
            if m:
                prefix_stack.append(m.group(1))
                continue
            if s == "})":
                if prefix_stack:
                    prefix_stack.pop()
                continue
            m2 = re.search(r'r\.(Get|Post|Put|Patch|Delete)\("([^"]+)"', s)
            if m2:
                method = m2.group(1).upper()
                path   = "".join(prefix_stack) + m2.group(2)
                registered.add((method, norm(path)))
    return registered


def load_spec_gaps(specs_dir, registered, in_scope_prefixes):
    gaps = []
    for specfile in sorted(glob.glob(os.path.join(specs_dir, "*.yml"))):
        with open(specfile) as f:
            try:
                spec = yaml.safe_load(f)
            except Exception as e:
                print(f"SKIP {specfile}: {e}", file=sys.stderr)
                continue
        service = os.path.basename(specfile).replace(".yml", "")
        for path, path_item in (spec.get("paths") or {}).items():
            # Check if path is in scope
            normed_path = norm(path)
            in_scope = any(
                re.sub(r'\{[^}]+\}', 'x', path).startswith(
                    re.sub(r'\{[^}]+\}', 'x', p).rstrip('/')
                )
                for p in in_scope_prefixes
            )
            if not in_scope:
                continue
            for method in ["get", "post", "put", "patch", "delete"]:
                op = (path_item or {}).get(method)
                if not op:
                    continue
                if (method.upper(), normed_path) not in registered:
                    op_id = op.get("operationId", "")
                    is_high = (method.upper(), normed_path) in HIGH_PRIORITY_OPS
                    gaps.append((service, method.upper(), path, normed_path, op_id, is_high))
    return gaps


def main():
    show_all = "--all" in sys.argv
    registered = load_registered_routes(HANDLERS_GO)
    gaps = load_spec_gaps(SPECS_DIR, registered, IN_SCOPE_PREFIXES)

    if not show_all:
        gaps = [g for g in gaps if g[5]]  # high priority only

    gaps.sort(key=lambda x: (not x[5], x[0], x[1], x[2]))

    prev_svc = None
    for service, method, path, normed, op_id, is_high in gaps:
        if service != prev_svc:
            print(f"\n=== {service} ===")
            prev_svc = service
        priority = "HIGH" if is_high else "    "
        print(f"  [{priority}] {method:6} {path}")
        if op_id:
            print(f"              ({op_id})")

    total = len(gaps)
    high  = sum(1 for g in gaps if g[5])
    print(f"\nRegistered routes: {len(registered)}")
    if show_all:
        print(f"Total gaps: {total}  (high priority: {high})")
    else:
        print(f"High-priority gaps: {high}  (run with --all to see all {total + len([g for g in load_spec_gaps(SPECS_DIR, registered, IN_SCOPE_PREFIXES) if not g[5]])} gaps)")


if __name__ == "__main__":
    main()
