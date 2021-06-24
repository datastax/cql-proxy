all: simplifiedcql_base_listener.go	simplifiedcql_lexer.go	simplifiedcql_listener.go  simplifiedcql_parser.go

simplifiedcql_base_listener.go	simplifiedcql_lexer.go	simplifiedcql_listener.go  simplifiedcql_parser.go:
	antlr4 -Dlanguage=Go SimplifiedCql.g4

clean:
	rm -f *.interp *.tokens simplifiedcql_base_listener.go	simplifiedcql_lexer.go	simplifiedcql_listener.go  simplifiedcql_parser.go
