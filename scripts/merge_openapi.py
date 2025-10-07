#!/usr/bin/env python3
import sys
import os
import yaml
from copy import deepcopy


def load_yaml(path):
    with open(path, 'r', encoding='utf-8') as f:
        return yaml.safe_load(f)


def deep_merge_dict(dst, src):
    for k, v in (src or {}).items():
        if k in dst and isinstance(dst[k], dict) and isinstance(v, dict):
            deep_merge_dict(dst[k], v)
        else:
            dst[k] = deepcopy(v)


def merge_specs(paths):
    out = {
        'openapi': '3.0.3',
        'info': {'title': 'FocusNest - All Services', 'version': '1.0.0'},
        'paths': {},
        'components': {'schemas': {}, 'securitySchemes': {}, 'parameters': {}, 'responses': {}},
        'tags': [],
    }

    # Optional default server override for Try-It-Out without editing individual specs
    default_server = os.environ.get('MERGE_DEFAULT_SERVER', '').strip()
    if default_server:
        out['servers'] = [{'url': default_server, 'description': 'Default'}]

    seen_tags = set()

    for p in paths:
        spec = load_yaml(p)
        # paths
        deep_merge_dict(out['paths'], spec.get('paths', {}))
        # components subsets
        comps = spec.get('components', {}) or {}
        for key in ['schemas', 'securitySchemes', 'parameters', 'responses']:
            deep_merge_dict(out['components'].setdefault(key, {}), comps.get(key, {}) or {})
        # tags
        for t in spec.get('tags', []) or []:
            name = t.get('name')
            if name and name not in seen_tags:
                out['tags'].append(t)
                seen_tags.add(name)
        # Do not merge servers from individual specs to keep output environment-agnostic
    return out


def main():
    if len(sys.argv) < 3:
        print('Usage: merge_openapi.py <output> <input1> [input2 ...]')
        sys.exit(1)
    output = sys.argv[1]
    inputs = sys.argv[2:]
    merged = merge_specs(inputs)
    with open(output, 'w', encoding='utf-8') as f:
        yaml.safe_dump(merged, f, sort_keys=False)
    print(f'Wrote {output} from {len(inputs)} inputs')


if __name__ == '__main__':
    main()


