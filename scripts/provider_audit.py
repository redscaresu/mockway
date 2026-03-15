#!/usr/bin/env python3
"""
provider_audit.py — find gaps between what the Terraform provider calls
and what mockway implements.

Strategy:
1. Parse the Scaleway SDK to build a map of  method_name → (HTTP_verb, path_template)
2. Grep the Terraform provider for every SDK method call
3. Normalize paths (replace runtime values with {x})
4. Cross-reference against mockway's registered routes
5. Report what the provider calls that mockway doesn't handle

Usage:
  python3 scripts/provider_audit.py [--all]
  --all: show every provider call, not just gaps
"""

import re
import sys
import os
from pathlib import Path
from collections import defaultdict

SDK_ROOT = Path("/tmp/scaleway-sdk-go/api")
PROVIDER_ROOT = Path("/tmp/terraform-provider-scaleway/internal")
MOCKWAY_HANDLERS = Path("handlers/handlers.go")

# SDK packages we care about (matches the services mockway covers)
SDK_SERVICES = {
    "instance/v1",
    "vpc/v1",
    "vpc/v2",
    "lb/v1",
    "k8s/v1",
    "rdb/v1",
    "redis/v1",
    "registry/v1",
    "iam/v1alpha1",
    "domain/v2beta1",
    "block/v1alpha1",
    "ipam/v1",
    "marketplace/v2",
}

# ── 1. Parse SDK: build method_name → (verb, path_template) ──────────────────

def normalize_sdk_path(path_str: str) -> str:
    """
    Convert a Go path string like:
      "/k8s/v1/regions/" + fmt.Sprint(req.Region) + "/clusters"
    into a normalised template:
      /k8s/v1/regions/{x}/clusters
    """
    # Remove Go string concatenation artifacts
    # Split on `+ ... +` segments, keep literal string parts
    parts = re.split(r'\s*\+\s*', path_str)
    result = []
    for part in parts:
        part = part.strip().strip('"')
        # Go expression like fmt.Sprint(...) or req.X or string(req.X)
        if not part.startswith('/') and part and not part.startswith('"'):
            result.append('{x}')
        else:
            result.append(part)
    normalized = ''.join(result)
    # Clean any remaining Go fragments
    normalized = re.sub(r'\bfmt\.Sprint\([^)]+\)', '{x}', normalized)
    normalized = re.sub(r'\bstring\([^)]+\)', '{x}', normalized)
    normalized = re.sub(r'\breq\.\w+', '{x}', normalized)
    # Collapse double slashes that might appear
    normalized = re.sub(r'//+', '/', normalized)
    return normalized


def parse_sdk_methods() -> dict:
    """Returns {qualified_method: (verb, path_template)}"""
    methods = {}

    for service in SDK_SERVICES:
        service_path = SDK_ROOT / service
        if not service_path.exists():
            continue
        for go_file in service_path.glob("*_sdk.go"):
            text = go_file.read_text(errors='replace')
            # Find all public API methods and their ScalewayRequest blocks
            # Pattern: func (s *API) MethodName(...) followed by scwReq := &scw.ScalewayRequest{...}
            fn_pattern = re.compile(
                r'func \(s \*(?:API|ZonedAPI)\) (\w+)\(.*?'
                r'scwReq\s*:=\s*&scw\.ScalewayRequest\s*\{(.*?)\}',
                re.DOTALL
            )
            for m in fn_pattern.finditer(text):
                fn_name = m.group(1)
                req_block = m.group(2)

                verb_m = re.search(r'Method:\s*"(\w+)"', req_block)
                path_m = re.search(r'Path:\s*(.+?)(?:,\s*$|\n)', req_block, re.MULTILINE)

                if not verb_m or not path_m:
                    continue

                verb = verb_m.group(1).upper()
                raw_path = path_m.group(1).strip().rstrip(',')
                path = normalize_sdk_path(raw_path)

                # Qualify with service so same method names don't collide
                key = f"{service}::{fn_name}"
                methods[key] = (verb, path)

    return methods


# SDK helper methods that don't correspond to real HTTP endpoints —
# they are polling wrappers (call Get* in a loop) or zone/region list utilities.
FALSE_POSITIVE_METHODS = {
    "WaitForCluster", "WaitForPool", "WaitForNode",
    "WaitForInstance", "WaitForRDBInstance", "WaitForReadReplica",
    "WaitForDatabaseBackup", "WaitForSnapshot", "WaitForImage",
    "WaitForLB", "WaitForIP", "WaitForPrivateNetwork", "WaitForVPC",
    "WaitForRedisCluster", "WaitForRegistryNamespace",
    "WaitForNamespace", "WaitForFunction", "WaitForContainer",
    "Zones", "Regions",  # utility helpers that return available regions/zones
}

# ── 2. Grep provider: find every SDK API method call ─────────────────────────

def find_provider_calls(sdk_methods: dict) -> dict:
    """
    Returns {qualified_method: (verb, path, [files_using_it])}
    Only methods actually called in the provider source.
    """
    # Build a reverse index: short method name → list of qualified keys
    short_to_qualified = defaultdict(list)
    for key in sdk_methods:
        short_name = key.split("::")[1]
        short_to_qualified[short_name].append(key)

    # Find all .go files in the provider
    provider_files = list(PROVIDER_ROOT.rglob("*.go"))

    called = defaultdict(set)  # qualified_key → set of files
    # Match patterns like: api.CreateCluster( or .CreateCluster(
    call_pattern = re.compile(r'\.(\w+)\(')

    for go_file in provider_files:
        try:
            text = go_file.read_text(errors='replace')
        except Exception:
            continue
        for m in call_pattern.finditer(text):
            method = m.group(1)
            if method in short_to_qualified:
                for qkey in short_to_qualified[method]:
                    called[qkey].add(str(go_file.relative_to(PROVIDER_ROOT)))

    result = {}
    for qkey, files in called.items():
        method_name = qkey.split("::")[1]
        if method_name in FALSE_POSITIVE_METHODS:
            continue
        verb, path = sdk_methods[qkey]
        result[qkey] = (verb, path, sorted(files))

    return result


# ── 3. Parse mockway routes ───────────────────────────────────────────────────

def load_mockway_routes() -> set:
    """Returns set of (verb, normalized_path)"""
    text = MOCKWAY_HANDLERS.read_text()
    routes = set()

    # Match r.Get("/path", ...) style route registrations
    route_pattern = re.compile(
        r'r\.(Get|Post|Put|Patch|Delete)\s*\(\s*"([^"]+)"',
        re.IGNORECASE
    )

    # We also need to accumulate the route prefixes from r.Route(...)
    # Simple approach: collect all raw registrations and their paths,
    # then resolve prefixes by parsing the nested structure.

    # Flatten: find all Route() prefix blocks and their contents
    # We'll do a two-pass: first collect all prefix+method+path combos.

    prefix_stack = []
    full_routes = []

    # Use a line-by-line state machine to track nesting
    route_decl = re.compile(r'r\.Route\s*\(\s*"([^"]+)"')
    leaf_decl = re.compile(r'r\.(Get|Post|Put|Patch|Delete)\s*\(\s*"([^"]+)"')
    open_brace = re.compile(r'\{')
    close_brace = re.compile(r'\}')

    depth = 0
    prefix_at_depth = {0: ""}

    for line in text.splitlines():
        stripped = line.strip()

        route_m = route_decl.search(stripped)
        leaf_m = leaf_decl.search(stripped)

        opens = len(open_brace.findall(stripped))
        closes = len(close_brace.findall(stripped))

        if route_m:
            seg = route_m.group(1)
            depth += opens - closes
            prefix_at_depth[depth] = prefix_at_depth.get(depth - 1, "") + seg
        elif leaf_m:
            verb = leaf_m.group(1).upper()
            seg = leaf_m.group(2)
            current_prefix = prefix_at_depth.get(depth, "")
            full_path = current_prefix + seg
            # Normalise {param} → {x}
            full_path = re.sub(r'\{[^}]+\}', '{x}', full_path)
            full_routes.append((verb, full_path))
        else:
            depth += opens - closes
            if depth < 0:
                depth = 0

    return set(full_routes)


# ── 4. Normalise SDK path to match mockway format ────────────────────────────

def normalize_for_compare(path: str) -> str:
    return re.sub(r'\{[^}]+\}', '{x}', path)


# ── 5. Main ───────────────────────────────────────────────────────────────────

def main():
    show_all = "--all" in sys.argv

    print("Parsing SDK methods...", file=sys.stderr)
    sdk_methods = parse_sdk_methods()
    print(f"  Found {len(sdk_methods)} SDK methods across {len(SDK_SERVICES)} services", file=sys.stderr)

    print("Scanning provider for calls...", file=sys.stderr)
    provider_calls = find_provider_calls(sdk_methods)
    print(f"  Provider calls {len(provider_calls)} distinct SDK methods", file=sys.stderr)

    print("Loading mockway routes...", file=sys.stderr)
    mockway_routes = load_mockway_routes()
    print(f"  Mockway has {len(mockway_routes)} routes", file=sys.stderr)

    # Find gaps
    gaps = []
    covered = []
    for qkey, (verb, path, files) in sorted(provider_calls.items()):
        norm = normalize_for_compare(path)
        if (verb, norm) not in mockway_routes:
            gaps.append((qkey, verb, path, files))
        else:
            covered.append((qkey, verb, path))

    print(f"\n{'='*60}")
    print(f"Provider calls:   {len(provider_calls)}")
    print(f"Covered:          {len(covered)}")
    print(f"GAPS (not in mockway): {len(gaps)}")
    print(f"{'='*60}\n")

    if gaps:
        # Group by service
        by_service = defaultdict(list)
        for qkey, verb, path, files in gaps:
            service = qkey.split("::")[0]
            method = qkey.split("::")[1]
            by_service[service].append((verb, path, method, files))

        for service in sorted(by_service):
            print(f"=== {service} ===")
            for verb, path, method, files in sorted(by_service[service]):
                print(f"  {verb:<7} {path}")
                print(f"          ({method})")
                if show_all:
                    for f in files[:3]:
                        print(f"          → {f}")
            print()
    else:
        print("No gaps found — mockway covers everything the provider calls.")

    if not show_all and gaps:
        print("Run with --all to see which provider files make each call.")


if __name__ == "__main__":
    main()
