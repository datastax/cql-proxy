package proxy

import (
	parser "cql-proxy/proxycore/antlr"
	"errors"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"strings"
)

var systemTables = []string{"local", "peers", "peers_v2"}

type aliasSelector struct {
	selector interface{}
	alias    string
}

type idSelector struct {
	name string
}

type starSelector struct{}

type countStarSelector struct {
	name string
}

type errorSelectStatement struct {
	err error
}

type selectStatement struct {
	table     string
	selectors []interface{}
}

type useStatement struct {
	keyspace string
}

func parse(keyspace string, query string) (handled bool, idempotent bool, stmt interface{}) {
	is := antlr.NewInputStream(query)
	lexer := parser.NewSimplifiedCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	cqlParser := parser.NewSimplifiedCqlParser(stream)
	listener := &queryListener{keyspace: keyspace}
	antlr.ParseTreeWalkerDefault.Walk(listener, cqlParser.CqlStatement())
	return listener.handled, listener.idempotent, listener.stmt
}

type queryListener struct {
	*parser.BaseSimplifiedCqlListener
	keyspace   string
	handled    bool
	idempotent bool
	stmt       interface{}
}

func isSystemTable(name string) bool {
	for _, table := range systemTables {
		if table == name {
			return true
		}
	}
	return false
}

func (l *queryListener) EnterSelectStatement(ctx *parser.SelectStatementContext) {
	l.idempotent = true

	tableNameCxt := ctx.TableName().(*parser.TableNameContext).QualifiedIdentifier().(*parser.QualifiedIdentifierContext)
	var keyspace string
	if tableNameCxt.KeyspaceName() != nil {
		keyspace = extractIdentifier(tableNameCxt.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))
	}

	table := extractIdentifier(tableNameCxt.Identifier().(*parser.IdentifierContext))

	if (keyspace == "system" || l.keyspace == "system") && isSystemTable(table) {
		l.handled = true

		selectStmt := &selectStatement{
			table:     table,
			selectors: make([]interface{}, 0),
		}

		selectClauseCtx := ctx.SelectClause().(*parser.SelectClauseContext)

		if selectClauseCtx.Selectors() != nil {
			selectorsCtx := selectClauseCtx.Selectors().(*parser.SelectorsContext)
			for _, selector := range selectorsCtx.AllSelector() {
				selectorCtx := selector.(*parser.SelectorContext)
				unaliasedSelector, err := extractUnaliasedSelector(selectorCtx.UnaliasedSelector().(*parser.UnaliasedSelectorContext))
				if err != nil {
					l.stmt = &errorSelectStatement{err}
					return // invalid selector
				}
				if selectorCtx.K_AS() != nil { // alias
					selectStmt.selectors = append(selectStmt.selectors, aliasSelector{
						selector: unaliasedSelector,
						alias:    extractIdentifier(selectorCtx.Identifier().(*parser.IdentifierContext)),
					})
				} else {
					selectStmt.selectors = append(selectStmt.selectors, unaliasedSelector)
				}
			}
		} else { // 'SELECT * ...'
			selectStmt.selectors = append(selectStmt.selectors, starSelector{})
		}

		l.stmt = selectStmt
	}
}

func (l *queryListener) EnterUseStatement(ctx *parser.UseStatementContext) {
	l.handled = true
	l.stmt = &useStatement{keyspace: extractIdentifier(ctx.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))}
}

func (l *queryListener) EnterInsertStatement(ctx *parser.InsertStatementContext) {
	// TODO: Check is idempotent
}

func (l *queryListener) EnterUpdateStatement(ctx *parser.UpdateStatementContext) {
	// TODO: Check is idempotent
}

func (l *queryListener) EnterDeleteStatement(ctx *parser.DeleteStatementContext) {
	// TODO: Check is idempotent
}

func extractUnaliasedSelector(selectorCtx *parser.UnaliasedSelectorContext) (interface{}, error) {
	if selectorCtx.K_COUNT() != nil {
		return countStarSelector{name: selectorCtx.GetText()}, nil
	} else if selectorCtx.Identifier() != nil {
		return idSelector{
			name: extractIdentifier(selectorCtx.Identifier().(*parser.IdentifierContext)),
		}, nil
	} else {
		return nil, errors.New("unsupported select clause for system table")
	}
}

func extractIdentifier(identifierCxt *parser.IdentifierContext) string {
	if unquotedIdentifier := identifierCxt.UNQUOTED_IDENTIFIER(); unquotedIdentifier != nil {
		return strings.ToLower(unquotedIdentifier.GetText())
	} else if quotedIdentifier := identifierCxt.QUOTED_IDENTIFIER(); quotedIdentifier != nil {
		identifier := quotedIdentifier.GetText()
		// remove surrounding quotes
		identifier = identifier[1 : len(identifier)-1]
		// handle escaped double-quotes
		identifier = strings.ReplaceAll(identifier, "\"\"", "\"")
		return identifier
	} else {
		return strings.ToLower(identifierCxt.UnreservedKeyword().GetText())
	}
}
