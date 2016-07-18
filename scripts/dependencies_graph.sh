#!/bin/bash -e
# Requires installation of package: graphviz

(echo "digraph G {"
go list -f '{{range .Imports}}{{printf "\t%q -> %q;\n" $.ImportPath .}}{{end}}' $(go list -f '{{join .Deps " "}}' github.com/smancke/guble ) github.com/smancke/guble
echo "}" ) | dot -Tsvg -o dependencies_graph.svg