#!/usr/bin/env python3
import tomli
from pathlib import Path
from typing import Set, List, Dict

def update_tags_md(tags: Set[str], providers: Set[str], provider_types: Set[str]):
    tags_path = Path(__file__).parent.parent / "docs" / "TAGS.md"
    if not tags_path.exists():
        tags_path.parent.mkdir(parents=True, exist_ok=True)

    sorted_tags = sorted(list(tags))
    abilities = {"image", "audio", "video", "reasoning", "long-context", "function-call", "agentic", "vision", "tools"}
    found_abilities = sorted([t for t in sorted_tags if t in abilities])
    all_sources = sorted(list(providers.union(provider_types)))
    features = {"cheap", "free", "fast", "chinese", "local", "open-source", "high-quality", "preview"}
    found_features = sorted([t for t in sorted_tags if t in features])

    new_header = (
        "functions:\n"
        f" - {', '.join(sorted_tags)}, \n\n"
        "abilities:\n"
        f" - {', '.join(found_abilities)}, \n\n"
        "sources:\n"
        f" - {', '.join(all_sources)}, ...\n\n"
        "features:\n"
        f" - {', '.join(found_features)}, \n\n"
    )

    existing = tags_path.read_text(encoding="utf-8")
    if "### Functions (功能)" in existing:
        rest = existing.split("### Functions (功能)", 1)[1]
        content = new_header + "### Functions (功能)" + rest
    else:
        content = new_header.rstrip() + "\n"
    tags_path.write_text(content, encoding="utf-8")
    print(f"Updated {tags_path}")

def main():
    router_toml_path = Path(__file__).parent.parent / "router.toml"
    if not router_toml_path.exists():
        print("Error: router.toml not found")
        return

    with open(router_toml_path, "rb") as f:
        config = tomli.load(f)

    all_tags = set()
    all_provider_names = set()
    all_provider_types = set()

    # Extract from providers
    for provider in config.get("providers", []):
        if "name" in provider:
            all_provider_names.add(provider["name"])
        if "type" in provider:
            all_provider_types.add(provider["type"])

    # Extract from models
    for model in config.get("models", []):
        tags = model.get("tags", [])
        for tag in tags:
            all_tags.add(tag)
        
        # Some providers might be mentioned in models but not in providers list (rare but possible)
        provider_name = model.get("provider")
        if provider_name:
            all_provider_names.add(provider_name)

    update_tags_md(all_tags, all_provider_names, all_provider_types)

    # Generate a summary for other docs
    print("\n--- Model Summary ---")
    provider_models = {}
    for model in config.get("models", []):
        p = model.get("provider", "unknown")
        if p not in provider_models:
            provider_models[p] = []
        provider_models[p].append(model.get("display_name") or model.get("name"))

    for p, models in sorted(provider_models.items()):
        print(f"- **{p}**: {', '.join(sorted(models))}")

    routing = config.get("routing", {})
    pairs = routing.get("pairs", [])
    if pairs:
        print("\n--- Routing Pairs ---")
        print(f"default_pair: {routing.get('default_pair', 'N/A')}")
        for p in pairs:
            name = p.get("name", "?")
            strong = p.get("strong_model", "?")
            weak = p.get("weak_model", "?")
            print(f"- {name}: strong={strong}, weak={weak}")

if __name__ == "__main__":
    main()

