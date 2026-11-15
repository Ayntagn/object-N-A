const encodeLabel = label => label.replace(/[\u0000-\u001f\u007f]/g, " ")
  .replace(/[!-/:-@\[-`{-~]/g, char => `#${char.charCodeAt(0)};`);
const declared = new Set();
function node(id) {
  const name = `n${id}`;
  if (declared.has(id)) return name;
  declared.add(id);
  return `${name}["${encodeLabel(input.labels[id])}"]`;
}
const lines = input.edges.map(({From, To, Reciprocal}) => `  ${node(From)} ${Reciprocal ? "---" : "-->"} ${node(To)}`);
const diagram = ["graph LR", ...lines].join("\n");
mermaid.initialize({
  startOnLoad: false,
  securityLevel: "strict",
  look: "classic",
  layout: "dagre",
  theme: "default",
  themeVariables: {fontSize: "20px"},
  flowchart: {curve: "linear", htmlLabels: false, minNodeWidth: 0, nodeSpacing: 50, rankSpacing: 50, padding: 15, wrappingWidth: 550},
  maxEdges: 1770,
  maxTextSize: 100000,
});

const main = document.querySelector("main");
const {svg} = await mermaid.render("vaultMap", diagram, main);
main.innerHTML = svg;
await document.fonts.ready;
const cards = Array(input.labels.length);
for (const group of main.querySelectorAll("g.node")) {
  const match = /^vaultMap-flowchart-n(\d+)-\d+$/.exec(group.id);
  if (!match) throw new Error("Unexpected flowchart node");
  const matrix = group.transform.baseVal.consolidate().matrix;
  const box = group.querySelector("rect").getBBox();
  const label = group.querySelector(".label").getBBox();
  cards[Number(match[1])] = {x: matrix.e + box.x, y: matrix.f + box.y,
    width: box.width, height: box.height, textWidth: label.width};
}
const paths = new Map([...main.querySelectorAll(".edgePaths path[data-id]")].map(path => [path.dataset.id, path]));
const arrows = input.edges.map(({From, To}) => {
  const path = paths.get(`L_n${From}_n${To}_0`);
  return JSON.parse(atob(path.dataset.points)).map(({x, y}) => [x, y]);
});
const measure = document.createElement("canvas").getContext("2d");
measure.font = "16px 'Trebuchet MS', sans-serif";
const legend = input.legend.map(label => measure.measureText(label).width);
return JSON.stringify({cards, arrows, legend});
