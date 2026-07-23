#!/usr/bin/env bash
# Lab stays in-tree at internal/lab (nested go.mod).
echo "ℹ️  Lab lives in-tree: internal/lab"
echo "   Nested module: root go test ./... skips it; use: make test-lab"
exit 0
