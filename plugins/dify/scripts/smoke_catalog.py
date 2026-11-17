"""Call openany_overview on the public MCP endpoint. Does not call run_api."""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import mcp_client


def main() -> int:
    result = mcp_client.call_tool("openany_overview", {})
    categories = [
        item.get("category")
        for item in result.get("apis_by_category") or []
        if isinstance(item, dict)
    ]
    modality_names = []
    modalities = result.get("models_by_modality") or []
    if isinstance(modalities, dict):
        modality_names = sorted(str(key) for key in modalities)
    elif isinstance(modalities, list):
        for item in modalities:
            if isinstance(item, dict) and item.get("modality"):
                modality_names.append(f"{item.get('modality')}:{item.get('count')}")
    print(f"api_count={result.get('api_count')}")
    print(f"model_count={result.get('model_count')}")
    print("categories=" + ",".join(str(item) for item in categories))
    print("modalities=" + ",".join(modality_names))
    if not isinstance(result.get("api_count"), int) or not isinstance(result.get("model_count"), int):
        print("smoke failed: overview did not return integer counts", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
