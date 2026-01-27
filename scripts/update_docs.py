#!/usr/bin/env python3
import tomli
from pathlib import Path
from typing import Set, List, Dict

def update_tags_md(tags: Set[str], providers: Set[str], provider_types: Set[str]):
    tags_path = Path(__file__).parent.parent / "TAGS.md"
    
    # Predefined lists from existing TAGS.md if possible, or just use what's found
    # Based on the read content of TAGS.md, it had specific sections.
    
    content = []
    content.append("functions:")
    # We don't necessarily know which tags are 'functions' vs 'abilities' vs 'features'
    # but we can try to categorize if we want, or just list them all.
    # For now, let's just update the lists with found values.
    
    sorted_tags = sorted(list(tags))
    content.append(f" - {', '.join(sorted_tags)}, ")
    content.append("")
    
    content.append("abilities:")
    # Some tags are traditionally abilities (image, audio, etc.)
    abilities = {"image", "audio", "video", "reasoning", "long-context", "function-call", "agentic", "vision", "tools"}
    found_abilities = sorted([t for t in sorted_tags if t in abilities])
    content.append(f" - {', '.join(found_abilities)}, ")
    content.append("")
    
    content.append("sources:")
    sorted_providers = sorted(list(providers))
    sorted_types = sorted(list(provider_types))
    # Combine provider names and types for sources
    all_sources = sorted(list(providers.union(provider_types)))
    content.append(f" - {', '.join(all_sources)}, ...")
    content.append("")
    
    content.append("features:")
    features = {"cheap", "free", "fast", "chinese", "local", "open-source", "high-quality", "preview"}
    found_features = sorted([t for t in sorted_tags if t in features])
    content.append(f" - {', '.join(found_features)}, ")
    
    tags_path.write_text("\n".join(content) + "\n")
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

if __name__ == "__main__":
    main()

