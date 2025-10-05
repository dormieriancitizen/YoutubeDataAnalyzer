import json
import numpy as np
import pandas as pd
import plotly.graph_objects as go
from pathlib import Path
from collections import Counter

p = Path("out/channels_by_index.json")
with p.open("r") as f:
    raw = json.load(f)

num_indices = len(raw)
indices = np.arange(1, num_indices + 1)

totals = Counter()
per_index = []
for index_list in raw:
    d = {}
    for obj in index_list:
        name = obj["Name"]
        hours = obj["WatchTime"] / 1e9 / 3600
        d[name] = hours
        totals[name] += hours
    per_index.append(d)

top_channels = [c for c, count in totals.most_common(100000000) if count > 2]

data = pd.DataFrame(index=indices, columns=top_channels + ["Other"])
for i, d in enumerate(per_index):
    row = {ch: d.get(ch, 0.0) for ch in top_channels}
    others = sum(d.values()) - sum(row.values())
    row["Other"] = max(0.0, others)
    data.iloc[i] = row

sorted_cols = data.sum(axis=0).sort_values(ascending=False).index
data = data[sorted_cols]

fig = go.Figure()

for ch in data.columns:
    fig.add_trace(
        go.Scatter(
            y=data[ch],
            mode="lines",
            stackgroup="one",
            # line_shape="spline",
            line=dict(width=0),
            name=ch,
            hovertemplate="%{fullData.name}: %{y:.2f}hrs <extra></extra>",
            hoveron="points+fills",
        )
    )

fig.update_layout(
    title="Stacked Area Chart of Watch Time",
    xaxis_title="Index",
    yaxis_title="Watch time (hours)",
    legend_title="Channel",
    hovermode="x unified",
)

fig.show()
