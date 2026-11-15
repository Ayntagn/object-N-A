# Survey readings

This chart reads `data/readings.csv`. It accompanies [[Project/Field Notes]].

```vega-lite
{
  "data": {"url": "data/readings.csv"},
  "mark": "bar",
  "encoding": {
    "x": {"field": "day", "type": "ordinal", "sort": ["Monday", "Tuesday", "Wednesday"]},
    "y": {"field": "reading", "type": "quantitative"}
  }
}
```
