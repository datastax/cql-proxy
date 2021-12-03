# Simplified CQL grammar

## How to generate the grammar files

First, install antlr 4.

On a Mac: `brew install antlr`.

Second, generate the Go files for the `SimplifiedCql` grammar:

    make

or:

    antlr -Dlanguage=Go antlr/SimplifiedCql.g4