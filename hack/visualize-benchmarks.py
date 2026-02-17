#!/usr/bin/env python3
"""
visualize-benchmarks.py - Generate charts from Go benchmark comparison data.

Usage:
    python3 hack/visualize-benchmarks.py benchmark-results/<timestamp>/comparison.json

Requires: matplotlib (pip install matplotlib)

Generates:
    - ns_per_op_comparison.png   - Bar chart comparing ns/op
    - pct_change.png             - Horizontal bar chart of % change
    - allocs_comparison.png      - Allocation comparison
    - category_summary.png       - Per-category average change
"""

import json
import os
import sys


def check_matplotlib():
    try:
        import matplotlib

        return True
    except ImportError:
        return False


def main():
    if len(sys.argv) < 2:
        print("Usage: python3 hack/visualize-benchmarks.py <comparison.json>")
        print("")
        print("Generate the JSON first:")
        print(
            "  go run hack/benchanalyze/main.go -old results/go-1.25.5.txt -new results/go-1.26.txt -output results/"
        )
        sys.exit(1)

    json_path = sys.argv[1]
    output_dir = os.path.join(os.path.dirname(json_path), "charts")
    os.makedirs(output_dir, exist_ok=True)

    if not check_matplotlib():
        print("matplotlib is required. Install with: pip install matplotlib")
        sys.exit(1)

    import matplotlib

    matplotlib.use("Agg")  # Non-interactive backend
    import matplotlib.pyplot as plt
    import matplotlib.ticker as mticker

    with open(json_path, "r") as f:
        report = json.load(f)

    old_label = report["old_label"]
    new_label = report["new_label"]
    entries = report["entries"]
    summary = report["summary"]

    if not entries:
        print("No benchmark entries found in the report.")
        sys.exit(1)

    # Prepare data
    names = [e["name"].replace("Benchmark", "") for e in entries]
    old_ns = [e["old_ns_per_op"] for e in entries]
    new_ns = [e["new_ns_per_op"] for e in entries]
    pct_changes = [e["ns_per_op_change_pct"] for e in entries]
    categories = [e["category"] for e in entries]

    # Color scheme
    color_old = "#4A90D9"  # Blue
    color_new = "#E74C3C"  # Red
    color_better = "#27AE60"  # Green
    color_worse = "#E74C3C"  # Red
    color_neutral = "#95A5A6"  # Gray

    # --- Chart 1: ns/op comparison (grouped bar chart) ---
    fig, ax = plt.subplots(figsize=(max(14, len(names) * 0.8), 8))
    x = range(len(names))
    width = 0.35

    bars1 = ax.bar(
        [i - width / 2 for i in x],
        old_ns,
        width,
        label=old_label,
        color=color_old,
        alpha=0.85,
    )
    bars2 = ax.bar(
        [i + width / 2 for i in x],
        new_ns,
        width,
        label=new_label,
        color=color_new,
        alpha=0.85,
    )

    ax.set_xlabel("Benchmark")
    ax.set_ylabel("ns/op (lower is better)")
    ax.set_title(f"Performance Comparison: {old_label} vs {new_label}")
    ax.set_xticks(list(x))
    ax.set_xticklabels(names, rotation=45, ha="right", fontsize=7)
    ax.legend()
    ax.grid(axis="y", alpha=0.3)
    ax.set_yscale("log")

    fig.tight_layout()
    chart_path = os.path.join(output_dir, "ns_per_op_comparison.png")
    fig.savefig(chart_path, dpi=150)
    plt.close(fig)
    print(f"Saved: {chart_path}")

    # --- Chart 2: % change horizontal bar chart ---
    # Sort by change
    sorted_indices = sorted(range(len(pct_changes)), key=lambda i: pct_changes[i])
    sorted_names = [names[i] for i in sorted_indices]
    sorted_changes = [pct_changes[i] for i in sorted_indices]
    bar_colors = [
        color_better if c < -1 else (color_worse if c > 1 else color_neutral)
        for c in sorted_changes
    ]

    fig, ax = plt.subplots(figsize=(12, max(6, len(names) * 0.35)))
    ax.barh(sorted_names, sorted_changes, color=bar_colors, alpha=0.85)
    ax.axvline(x=0, color="black", linewidth=0.8)
    ax.set_xlabel("% Change (negative = improvement)")
    ax.set_title(f"Performance Change: {old_label} -> {new_label}")
    ax.grid(axis="x", alpha=0.3)

    # Add value labels
    for i, (v, name) in enumerate(zip(sorted_changes, sorted_names)):
        offset = 0.5 if v >= 0 else -0.5
        ax.text(v + offset, i, f"{v:+.1f}%", va="center", fontsize=7)

    fig.tight_layout()
    chart_path = os.path.join(output_dir, "pct_change.png")
    fig.savefig(chart_path, dpi=150)
    plt.close(fig)
    print(f"Saved: {chart_path}")

    # --- Chart 3: Allocation comparison ---
    old_bytes = [e["old_bytes_per_op"] for e in entries]
    new_bytes = [e["new_bytes_per_op"] for e in entries]
    old_allocs = [e["old_allocs_per_op"] for e in entries]
    new_allocs = [e["new_allocs_per_op"] for e in entries]

    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(18, max(6, len(names) * 0.3)))

    # B/op
    y_pos = range(len(names))
    ax1.barh(
        [i - 0.15 for i in y_pos],
        old_bytes,
        0.3,
        label=old_label,
        color=color_old,
        alpha=0.85,
    )
    ax1.barh(
        [i + 0.15 for i in y_pos],
        new_bytes,
        0.3,
        label=new_label,
        color=color_new,
        alpha=0.85,
    )
    ax1.set_yticks(list(y_pos))
    ax1.set_yticklabels(names, fontsize=7)
    ax1.set_xlabel("B/op (lower is better)")
    ax1.set_title("Bytes per Operation")
    ax1.legend(fontsize=8)
    ax1.grid(axis="x", alpha=0.3)

    # allocs/op
    ax2.barh(
        [i - 0.15 for i in y_pos],
        old_allocs,
        0.3,
        label=old_label,
        color=color_old,
        alpha=0.85,
    )
    ax2.barh(
        [i + 0.15 for i in y_pos],
        new_allocs,
        0.3,
        label=new_label,
        color=color_new,
        alpha=0.85,
    )
    ax2.set_yticks(list(y_pos))
    ax2.set_yticklabels(names, fontsize=7)
    ax2.set_xlabel("allocs/op (lower is better)")
    ax2.set_title("Allocations per Operation")
    ax2.legend(fontsize=8)
    ax2.grid(axis="x", alpha=0.3)

    fig.suptitle(
        f"Memory Allocation Comparison: {old_label} vs {new_label}", fontsize=14
    )
    fig.tight_layout()
    chart_path = os.path.join(output_dir, "allocs_comparison.png")
    fig.savefig(chart_path, dpi=150)
    plt.close(fig)
    print(f"Saved: {chart_path}")

    # --- Chart 4: Category summary ---
    cat_changes = {}
    cat_counts = {}
    for e in entries:
        cat = e["category"]
        cat_changes.setdefault(cat, []).append(e["ns_per_op_change_pct"])
        cat_counts[cat] = cat_counts.get(cat, 0) + 1

    cat_names = sorted(cat_changes.keys())
    cat_avgs = [sum(cat_changes[c]) / len(cat_changes[c]) for c in cat_names]
    cat_labels = [f"{c} (n={cat_counts[c]})" for c in cat_names]
    cat_colors = [
        color_better if v < -1 else (color_worse if v > 1 else color_neutral)
        for v in cat_avgs
    ]

    fig, ax = plt.subplots(figsize=(10, max(4, len(cat_names) * 0.6)))
    ax.barh(cat_labels, cat_avgs, color=cat_colors, alpha=0.85)
    ax.axvline(x=0, color="black", linewidth=0.8)
    ax.set_xlabel("Average % Change (negative = improvement)")
    ax.set_title(f"Performance Change by Category: {old_label} -> {new_label}")
    ax.grid(axis="x", alpha=0.3)

    for i, v in enumerate(cat_avgs):
        offset = 0.3 if v >= 0 else -0.3
        ax.text(v + offset, i, f"{v:+.1f}%", va="center", fontsize=9, fontweight="bold")

    fig.tight_layout()
    chart_path = os.path.join(output_dir, "category_summary.png")
    fig.savefig(chart_path, dpi=150)
    plt.close(fig)
    print(f"Saved: {chart_path}")

    # --- Summary output ---
    print(f"\nAll charts saved to: {output_dir}/")
    print(f"\nSummary:")
    print(f"  {new_label} wins: {summary['new_wins']}")
    print(f"  {old_label} wins: {summary['old_wins']}")
    print(f"  Ties:       {summary['ties']}")
    print(f"  Avg change: {summary['avg_ns_per_op_change_pct']:+.2f}%")


if __name__ == "__main__":
    main()
