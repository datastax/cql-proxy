all: cql_lexer.go

cql_lexer.go:
	antlr4 -Dlanguage=Go CqlLexer.g4

clean:
	rm -f *.interp *.tokens cql_lexer.go
