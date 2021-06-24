package proxy

import (
	parser "cql-proxy/proxycore/antlr"
	"errors"
	"github.com/antlr/antlr4/runtime/Go/antlr"
	"strings"
)

var systemTables = []string{"local", "peers", "peers_v2"}

type statement interface {
	isIdempotent() bool
	isHandled() bool
}

type notHandledStatement struct {
	idempotent bool
}

func (s *notHandledStatement) isIdempotent() bool {
	return s.idempotent
}

func (s *notHandledStatement) isHandled() bool {
	return false
}

var notIdempotentStmt = &notHandledStatement{idempotent: false}
var idempotentStmt = &notHandledStatement{idempotent: true}

type aliasSelector struct {
	selector interface{}
	alias    string
}

type idSelector struct {
	name string
}

type starSelector struct {
}

type countStarSelector struct {
}

type errorSelectStatement struct {
	err error
}

func (e errorSelectStatement) isIdempotent() bool {
	return false
}

func (e errorSelectStatement) isHandled() bool {
	return true
}

type selectStatement struct {
	table     string
	selectors []interface{}
}

func (s selectStatement) isHandled() bool {
	return true
}

func (s selectStatement) isIdempotent() bool {
	return true
}

type useStatement struct {
	keyspace string
}

func (s useStatement) isHandled() bool {
	return true
}

func (s useStatement) isIdempotent() bool {
	return false
}

func parse(keyspace string, query string) statement {
	is := antlr.NewInputStream(query)
	lexer := parser.NewSimplifiedCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	cqlParser := parser.NewSimplifiedCqlParser(stream)
	listener := &queryListener{keyspace: keyspace, stmt: notIdempotentStmt}
	antlr.ParseTreeWalkerDefault.Walk(listener, cqlParser.CqlStatement())
	return listener.stmt
}

type queryListener struct {
	*parser.BaseSimplifiedCqlListener
	keyspace string
	stmt     statement
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
	tableNameCxt := ctx.TableName().(*parser.TableNameContext).QualifiedIdentifier().(*parser.QualifiedIdentifierContext)
	var keyspace string
	if tableNameCxt.KeyspaceName() != nil {
		keyspace = extractIdentifier(tableNameCxt.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))
	}

	table := extractIdentifier(tableNameCxt.Identifier().(*parser.IdentifierContext))

	if (keyspace == "system" || l.keyspace == "system") && isSystemTable(table) {
		selectStmt := selectStatement{
			table:     table,
			selectors: make([]interface{}, 0),
		}

		selectClauseCtx := ctx.SelectClause().(*parser.SelectClauseContext)

		selectorsCtx := selectClauseCtx.Selectors().(*parser.SelectorsContext)
		if selectorsCtx != nil {
			for _, selector := range selectorsCtx.AllSelector() {
				selectorCtx := selector.(*parser.SelectorContext)
				unaliasedSelector, err := extractUnaliasedSelector(selectorCtx.UnaliasedSelector().(*parser.UnaliasedSelectorContext))
				if err != nil {
					l.stmt = errorSelectStatement{err}
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
	} else {
		l.stmt = idempotentStmt
	}

}

func (l *queryListener) EnterUseStatement(ctx *parser.UseStatementContext) {
	l.stmt = useStatement{keyspace: extractIdentifier(ctx.KeyspaceName().(*parser.KeyspaceNameContext).Identifier().(*parser.IdentifierContext))}
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
		return countStarSelector{}, nil
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
