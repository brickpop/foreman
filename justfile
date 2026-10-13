[private]
default: help

# List the available recipes
help:
    @just --list --unsorted

# Run any wiki command against the knowledge base, e.g. `just wiki status`
[group('wiki')]
wiki *args:
    wiki --root wiki {{args}}

# Health check of the knowledge base (errors fail; broken links are warnings)
[group('wiki')]
check:
    wiki --root wiki check
