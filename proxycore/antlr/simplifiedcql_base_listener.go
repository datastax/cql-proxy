// Code generated from SimplifiedCql.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // SimplifiedCql

import "github.com/antlr/antlr4/runtime/Go/antlr"

// BaseSimplifiedCqlListener is a complete listener for a parse tree produced by SimplifiedCqlParser.
type BaseSimplifiedCqlListener struct{}

var _ SimplifiedCqlListener = &BaseSimplifiedCqlListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseSimplifiedCqlListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseSimplifiedCqlListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseSimplifiedCqlListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseSimplifiedCqlListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterCqlStatement is called when production cqlStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterCqlStatement(ctx *CqlStatementContext) {}

// ExitCqlStatement is called when production cqlStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitCqlStatement(ctx *CqlStatementContext) {}

// EnterInsertStatement is called when production insertStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterInsertStatement(ctx *InsertStatementContext) {}

// ExitInsertStatement is called when production insertStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitInsertStatement(ctx *InsertStatementContext) {}

// EnterUpdateStatement is called when production updateStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateStatement(ctx *UpdateStatementContext) {}

// ExitUpdateStatement is called when production updateStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateStatement(ctx *UpdateStatementContext) {}

// EnterUpdateOperations is called when production updateOperations is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperations(ctx *UpdateOperationsContext) {}

// ExitUpdateOperations is called when production updateOperations is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperations(ctx *UpdateOperationsContext) {}

// EnterUpdateOperation is called when production updateOperation is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperation(ctx *UpdateOperationContext) {}

// ExitUpdateOperation is called when production updateOperation is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperation(ctx *UpdateOperationContext) {}

// EnterUpdateOperatorAddLeft is called when production updateOperatorAddLeft is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperatorAddLeft(ctx *UpdateOperatorAddLeftContext) {}

// ExitUpdateOperatorAddLeft is called when production updateOperatorAddLeft is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperatorAddLeft(ctx *UpdateOperatorAddLeftContext) {}

// EnterUpdateOperatorAddRight is called when production updateOperatorAddRight is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperatorAddRight(ctx *UpdateOperatorAddRightContext) {}

// ExitUpdateOperatorAddRight is called when production updateOperatorAddRight is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperatorAddRight(ctx *UpdateOperatorAddRightContext) {}

// EnterUpdateOperatorSubtract is called when production updateOperatorSubtract is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperatorSubtract(ctx *UpdateOperatorSubtractContext) {}

// ExitUpdateOperatorSubtract is called when production updateOperatorSubtract is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperatorSubtract(ctx *UpdateOperatorSubtractContext) {}

// EnterUpdateOperatorAddAssign is called when production updateOperatorAddAssign is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperatorAddAssign(ctx *UpdateOperatorAddAssignContext) {
}

// ExitUpdateOperatorAddAssign is called when production updateOperatorAddAssign is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperatorAddAssign(ctx *UpdateOperatorAddAssignContext) {
}

// EnterUpdateOperatorSubtractAssign is called when production updateOperatorSubtractAssign is entered.
func (s *BaseSimplifiedCqlListener) EnterUpdateOperatorSubtractAssign(ctx *UpdateOperatorSubtractAssignContext) {
}

// ExitUpdateOperatorSubtractAssign is called when production updateOperatorSubtractAssign is exited.
func (s *BaseSimplifiedCqlListener) ExitUpdateOperatorSubtractAssign(ctx *UpdateOperatorSubtractAssignContext) {
}

// EnterDeleteStatement is called when production deleteStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterDeleteStatement(ctx *DeleteStatementContext) {}

// ExitDeleteStatement is called when production deleteStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitDeleteStatement(ctx *DeleteStatementContext) {}

// EnterDeleteOperations is called when production deleteOperations is entered.
func (s *BaseSimplifiedCqlListener) EnterDeleteOperations(ctx *DeleteOperationsContext) {}

// ExitDeleteOperations is called when production deleteOperations is exited.
func (s *BaseSimplifiedCqlListener) ExitDeleteOperations(ctx *DeleteOperationsContext) {}

// EnterDeleteOperation is called when production deleteOperation is entered.
func (s *BaseSimplifiedCqlListener) EnterDeleteOperation(ctx *DeleteOperationContext) {}

// ExitDeleteOperation is called when production deleteOperation is exited.
func (s *BaseSimplifiedCqlListener) ExitDeleteOperation(ctx *DeleteOperationContext) {}

// EnterDeleteOperationElement is called when production deleteOperationElement is entered.
func (s *BaseSimplifiedCqlListener) EnterDeleteOperationElement(ctx *DeleteOperationElementContext) {}

// ExitDeleteOperationElement is called when production deleteOperationElement is exited.
func (s *BaseSimplifiedCqlListener) ExitDeleteOperationElement(ctx *DeleteOperationElementContext) {}

// EnterBatchStatement is called when production batchStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterBatchStatement(ctx *BatchStatementContext) {}

// ExitBatchStatement is called when production batchStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitBatchStatement(ctx *BatchStatementContext) {}

// EnterBatchChildStatement is called when production batchChildStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterBatchChildStatement(ctx *BatchChildStatementContext) {}

// ExitBatchChildStatement is called when production batchChildStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitBatchChildStatement(ctx *BatchChildStatementContext) {}

// EnterSelectStatement is called when production selectStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterSelectStatement(ctx *SelectStatementContext) {}

// ExitSelectStatement is called when production selectStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitSelectStatement(ctx *SelectStatementContext) {}

// EnterSelectClause is called when production selectClause is entered.
func (s *BaseSimplifiedCqlListener) EnterSelectClause(ctx *SelectClauseContext) {}

// ExitSelectClause is called when production selectClause is exited.
func (s *BaseSimplifiedCqlListener) ExitSelectClause(ctx *SelectClauseContext) {}

// EnterSelectors is called when production selectors is entered.
func (s *BaseSimplifiedCqlListener) EnterSelectors(ctx *SelectorsContext) {}

// ExitSelectors is called when production selectors is exited.
func (s *BaseSimplifiedCqlListener) ExitSelectors(ctx *SelectorsContext) {}

// EnterSelector is called when production selector is entered.
func (s *BaseSimplifiedCqlListener) EnterSelector(ctx *SelectorContext) {}

// ExitSelector is called when production selector is exited.
func (s *BaseSimplifiedCqlListener) ExitSelector(ctx *SelectorContext) {}

// EnterUnaliasedSelector is called when production unaliasedSelector is entered.
func (s *BaseSimplifiedCqlListener) EnterUnaliasedSelector(ctx *UnaliasedSelectorContext) {}

// ExitUnaliasedSelector is called when production unaliasedSelector is exited.
func (s *BaseSimplifiedCqlListener) ExitUnaliasedSelector(ctx *UnaliasedSelectorContext) {}

// EnterUseStatement is called when production useStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterUseStatement(ctx *UseStatementContext) {}

// ExitUseStatement is called when production useStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitUseStatement(ctx *UseStatementContext) {}

// EnterOrderByClause is called when production orderByClause is entered.
func (s *BaseSimplifiedCqlListener) EnterOrderByClause(ctx *OrderByClauseContext) {}

// ExitOrderByClause is called when production orderByClause is exited.
func (s *BaseSimplifiedCqlListener) ExitOrderByClause(ctx *OrderByClauseContext) {}

// EnterOrderings is called when production orderings is entered.
func (s *BaseSimplifiedCqlListener) EnterOrderings(ctx *OrderingsContext) {}

// ExitOrderings is called when production orderings is exited.
func (s *BaseSimplifiedCqlListener) ExitOrderings(ctx *OrderingsContext) {}

// EnterOrdering is called when production ordering is entered.
func (s *BaseSimplifiedCqlListener) EnterOrdering(ctx *OrderingContext) {}

// ExitOrdering is called when production ordering is exited.
func (s *BaseSimplifiedCqlListener) ExitOrdering(ctx *OrderingContext) {}

// EnterGroupByClause is called when production groupByClause is entered.
func (s *BaseSimplifiedCqlListener) EnterGroupByClause(ctx *GroupByClauseContext) {}

// ExitGroupByClause is called when production groupByClause is exited.
func (s *BaseSimplifiedCqlListener) ExitGroupByClause(ctx *GroupByClauseContext) {}

// EnterPerPartitionLimitClause is called when production perPartitionLimitClause is entered.
func (s *BaseSimplifiedCqlListener) EnterPerPartitionLimitClause(ctx *PerPartitionLimitClauseContext) {
}

// ExitPerPartitionLimitClause is called when production perPartitionLimitClause is exited.
func (s *BaseSimplifiedCqlListener) ExitPerPartitionLimitClause(ctx *PerPartitionLimitClauseContext) {
}

// EnterLimitClause is called when production limitClause is entered.
func (s *BaseSimplifiedCqlListener) EnterLimitClause(ctx *LimitClauseContext) {}

// ExitLimitClause is called when production limitClause is exited.
func (s *BaseSimplifiedCqlListener) ExitLimitClause(ctx *LimitClauseContext) {}

// EnterUsingClause is called when production usingClause is entered.
func (s *BaseSimplifiedCqlListener) EnterUsingClause(ctx *UsingClauseContext) {}

// ExitUsingClause is called when production usingClause is exited.
func (s *BaseSimplifiedCqlListener) ExitUsingClause(ctx *UsingClauseContext) {}

// EnterTimestamp is called when production timestamp is entered.
func (s *BaseSimplifiedCqlListener) EnterTimestamp(ctx *TimestampContext) {}

// ExitTimestamp is called when production timestamp is exited.
func (s *BaseSimplifiedCqlListener) ExitTimestamp(ctx *TimestampContext) {}

// EnterTtl is called when production ttl is entered.
func (s *BaseSimplifiedCqlListener) EnterTtl(ctx *TtlContext) {}

// ExitTtl is called when production ttl is exited.
func (s *BaseSimplifiedCqlListener) ExitTtl(ctx *TtlContext) {}

// EnterConditions is called when production conditions is entered.
func (s *BaseSimplifiedCqlListener) EnterConditions(ctx *ConditionsContext) {}

// ExitConditions is called when production conditions is exited.
func (s *BaseSimplifiedCqlListener) ExitConditions(ctx *ConditionsContext) {}

// EnterCondition is called when production condition is entered.
func (s *BaseSimplifiedCqlListener) EnterCondition(ctx *ConditionContext) {}

// ExitCondition is called when production condition is exited.
func (s *BaseSimplifiedCqlListener) ExitCondition(ctx *ConditionContext) {}

// EnterWhereClause is called when production whereClause is entered.
func (s *BaseSimplifiedCqlListener) EnterWhereClause(ctx *WhereClauseContext) {}

// ExitWhereClause is called when production whereClause is exited.
func (s *BaseSimplifiedCqlListener) ExitWhereClause(ctx *WhereClauseContext) {}

// EnterRelation is called when production relation is entered.
func (s *BaseSimplifiedCqlListener) EnterRelation(ctx *RelationContext) {}

// ExitRelation is called when production relation is exited.
func (s *BaseSimplifiedCqlListener) ExitRelation(ctx *RelationContext) {}

// EnterOperator is called when production operator is entered.
func (s *BaseSimplifiedCqlListener) EnterOperator(ctx *OperatorContext) {}

// ExitOperator is called when production operator is exited.
func (s *BaseSimplifiedCqlListener) ExitOperator(ctx *OperatorContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseSimplifiedCqlListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseSimplifiedCqlListener) ExitLiteral(ctx *LiteralContext) {}

// EnterPrimitiveLiteral is called when production primitiveLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterPrimitiveLiteral(ctx *PrimitiveLiteralContext) {}

// ExitPrimitiveLiteral is called when production primitiveLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitPrimitiveLiteral(ctx *PrimitiveLiteralContext) {}

// EnterCollectionLiteral is called when production collectionLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterCollectionLiteral(ctx *CollectionLiteralContext) {}

// ExitCollectionLiteral is called when production collectionLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitCollectionLiteral(ctx *CollectionLiteralContext) {}

// EnterListLiteral is called when production listLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterListLiteral(ctx *ListLiteralContext) {}

// ExitListLiteral is called when production listLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitListLiteral(ctx *ListLiteralContext) {}

// EnterSetLiteral is called when production setLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterSetLiteral(ctx *SetLiteralContext) {}

// ExitSetLiteral is called when production setLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitSetLiteral(ctx *SetLiteralContext) {}

// EnterMapLiteral is called when production mapLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterMapLiteral(ctx *MapLiteralContext) {}

// ExitMapLiteral is called when production mapLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitMapLiteral(ctx *MapLiteralContext) {}

// EnterMapEntries is called when production mapEntries is entered.
func (s *BaseSimplifiedCqlListener) EnterMapEntries(ctx *MapEntriesContext) {}

// ExitMapEntries is called when production mapEntries is exited.
func (s *BaseSimplifiedCqlListener) ExitMapEntries(ctx *MapEntriesContext) {}

// EnterMapEntry is called when production mapEntry is entered.
func (s *BaseSimplifiedCqlListener) EnterMapEntry(ctx *MapEntryContext) {}

// ExitMapEntry is called when production mapEntry is exited.
func (s *BaseSimplifiedCqlListener) ExitMapEntry(ctx *MapEntryContext) {}

// EnterTupleLiterals is called when production tupleLiterals is entered.
func (s *BaseSimplifiedCqlListener) EnterTupleLiterals(ctx *TupleLiteralsContext) {}

// ExitTupleLiterals is called when production tupleLiterals is exited.
func (s *BaseSimplifiedCqlListener) ExitTupleLiterals(ctx *TupleLiteralsContext) {}

// EnterTupleLiteral is called when production tupleLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterTupleLiteral(ctx *TupleLiteralContext) {}

// ExitTupleLiteral is called when production tupleLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitTupleLiteral(ctx *TupleLiteralContext) {}

// EnterUdtLiteral is called when production udtLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterUdtLiteral(ctx *UdtLiteralContext) {}

// ExitUdtLiteral is called when production udtLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitUdtLiteral(ctx *UdtLiteralContext) {}

// EnterFieldLiterals is called when production fieldLiterals is entered.
func (s *BaseSimplifiedCqlListener) EnterFieldLiterals(ctx *FieldLiteralsContext) {}

// ExitFieldLiterals is called when production fieldLiterals is exited.
func (s *BaseSimplifiedCqlListener) ExitFieldLiterals(ctx *FieldLiteralsContext) {}

// EnterFieldLiteral is called when production fieldLiteral is entered.
func (s *BaseSimplifiedCqlListener) EnterFieldLiteral(ctx *FieldLiteralContext) {}

// ExitFieldLiteral is called when production fieldLiteral is exited.
func (s *BaseSimplifiedCqlListener) ExitFieldLiteral(ctx *FieldLiteralContext) {}

// EnterFunctionCall is called when production functionCall is entered.
func (s *BaseSimplifiedCqlListener) EnterFunctionCall(ctx *FunctionCallContext) {}

// ExitFunctionCall is called when production functionCall is exited.
func (s *BaseSimplifiedCqlListener) ExitFunctionCall(ctx *FunctionCallContext) {}

// EnterFunctionArgs is called when production functionArgs is entered.
func (s *BaseSimplifiedCqlListener) EnterFunctionArgs(ctx *FunctionArgsContext) {}

// ExitFunctionArgs is called when production functionArgs is exited.
func (s *BaseSimplifiedCqlListener) ExitFunctionArgs(ctx *FunctionArgsContext) {}

// EnterFunctionArg is called when production functionArg is entered.
func (s *BaseSimplifiedCqlListener) EnterFunctionArg(ctx *FunctionArgContext) {}

// ExitFunctionArg is called when production functionArg is exited.
func (s *BaseSimplifiedCqlListener) ExitFunctionArg(ctx *FunctionArgContext) {}

// EnterBindMarkers is called when production bindMarkers is entered.
func (s *BaseSimplifiedCqlListener) EnterBindMarkers(ctx *BindMarkersContext) {}

// ExitBindMarkers is called when production bindMarkers is exited.
func (s *BaseSimplifiedCqlListener) ExitBindMarkers(ctx *BindMarkersContext) {}

// EnterBindMarker is called when production bindMarker is entered.
func (s *BaseSimplifiedCqlListener) EnterBindMarker(ctx *BindMarkerContext) {}

// ExitBindMarker is called when production bindMarker is exited.
func (s *BaseSimplifiedCqlListener) ExitBindMarker(ctx *BindMarkerContext) {}

// EnterPositionalBindMarker is called when production positionalBindMarker is entered.
func (s *BaseSimplifiedCqlListener) EnterPositionalBindMarker(ctx *PositionalBindMarkerContext) {}

// ExitPositionalBindMarker is called when production positionalBindMarker is exited.
func (s *BaseSimplifiedCqlListener) ExitPositionalBindMarker(ctx *PositionalBindMarkerContext) {}

// EnterNamedBindMarker is called when production namedBindMarker is entered.
func (s *BaseSimplifiedCqlListener) EnterNamedBindMarker(ctx *NamedBindMarkerContext) {}

// ExitNamedBindMarker is called when production namedBindMarker is exited.
func (s *BaseSimplifiedCqlListener) ExitNamedBindMarker(ctx *NamedBindMarkerContext) {}

// EnterTerms is called when production terms is entered.
func (s *BaseSimplifiedCqlListener) EnterTerms(ctx *TermsContext) {}

// ExitTerms is called when production terms is exited.
func (s *BaseSimplifiedCqlListener) ExitTerms(ctx *TermsContext) {}

// EnterTerm is called when production term is entered.
func (s *BaseSimplifiedCqlListener) EnterTerm(ctx *TermContext) {}

// ExitTerm is called when production term is exited.
func (s *BaseSimplifiedCqlListener) ExitTerm(ctx *TermContext) {}

// EnterTypeCast is called when production typeCast is entered.
func (s *BaseSimplifiedCqlListener) EnterTypeCast(ctx *TypeCastContext) {}

// ExitTypeCast is called when production typeCast is exited.
func (s *BaseSimplifiedCqlListener) ExitTypeCast(ctx *TypeCastContext) {}

// EnterCqlType is called when production cqlType is entered.
func (s *BaseSimplifiedCqlListener) EnterCqlType(ctx *CqlTypeContext) {}

// ExitCqlType is called when production cqlType is exited.
func (s *BaseSimplifiedCqlListener) ExitCqlType(ctx *CqlTypeContext) {}

// EnterPrimitiveType is called when production primitiveType is entered.
func (s *BaseSimplifiedCqlListener) EnterPrimitiveType(ctx *PrimitiveTypeContext) {}

// ExitPrimitiveType is called when production primitiveType is exited.
func (s *BaseSimplifiedCqlListener) ExitPrimitiveType(ctx *PrimitiveTypeContext) {}

// EnterCollectionType is called when production collectionType is entered.
func (s *BaseSimplifiedCqlListener) EnterCollectionType(ctx *CollectionTypeContext) {}

// ExitCollectionType is called when production collectionType is exited.
func (s *BaseSimplifiedCqlListener) ExitCollectionType(ctx *CollectionTypeContext) {}

// EnterTupleType is called when production tupleType is entered.
func (s *BaseSimplifiedCqlListener) EnterTupleType(ctx *TupleTypeContext) {}

// ExitTupleType is called when production tupleType is exited.
func (s *BaseSimplifiedCqlListener) ExitTupleType(ctx *TupleTypeContext) {}

// EnterTableName is called when production tableName is entered.
func (s *BaseSimplifiedCqlListener) EnterTableName(ctx *TableNameContext) {}

// ExitTableName is called when production tableName is exited.
func (s *BaseSimplifiedCqlListener) ExitTableName(ctx *TableNameContext) {}

// EnterFunctionName is called when production functionName is entered.
func (s *BaseSimplifiedCqlListener) EnterFunctionName(ctx *FunctionNameContext) {}

// ExitFunctionName is called when production functionName is exited.
func (s *BaseSimplifiedCqlListener) ExitFunctionName(ctx *FunctionNameContext) {}

// EnterUserTypeName is called when production userTypeName is entered.
func (s *BaseSimplifiedCqlListener) EnterUserTypeName(ctx *UserTypeNameContext) {}

// ExitUserTypeName is called when production userTypeName is exited.
func (s *BaseSimplifiedCqlListener) ExitUserTypeName(ctx *UserTypeNameContext) {}

// EnterKeyspaceName is called when production keyspaceName is entered.
func (s *BaseSimplifiedCqlListener) EnterKeyspaceName(ctx *KeyspaceNameContext) {}

// ExitKeyspaceName is called when production keyspaceName is exited.
func (s *BaseSimplifiedCqlListener) ExitKeyspaceName(ctx *KeyspaceNameContext) {}

// EnterQualifiedIdentifier is called when production qualifiedIdentifier is entered.
func (s *BaseSimplifiedCqlListener) EnterQualifiedIdentifier(ctx *QualifiedIdentifierContext) {}

// ExitQualifiedIdentifier is called when production qualifiedIdentifier is exited.
func (s *BaseSimplifiedCqlListener) ExitQualifiedIdentifier(ctx *QualifiedIdentifierContext) {}

// EnterIdentifiers is called when production identifiers is entered.
func (s *BaseSimplifiedCqlListener) EnterIdentifiers(ctx *IdentifiersContext) {}

// ExitIdentifiers is called when production identifiers is exited.
func (s *BaseSimplifiedCqlListener) ExitIdentifiers(ctx *IdentifiersContext) {}

// EnterIdentifier is called when production identifier is entered.
func (s *BaseSimplifiedCqlListener) EnterIdentifier(ctx *IdentifierContext) {}

// ExitIdentifier is called when production identifier is exited.
func (s *BaseSimplifiedCqlListener) ExitIdentifier(ctx *IdentifierContext) {}

// EnterUnreservedKeyword is called when production unreservedKeyword is entered.
func (s *BaseSimplifiedCqlListener) EnterUnreservedKeyword(ctx *UnreservedKeywordContext) {}

// ExitUnreservedKeyword is called when production unreservedKeyword is exited.
func (s *BaseSimplifiedCqlListener) ExitUnreservedKeyword(ctx *UnreservedKeywordContext) {}

// EnterUnrecognizedStatement is called when production unrecognizedStatement is entered.
func (s *BaseSimplifiedCqlListener) EnterUnrecognizedStatement(ctx *UnrecognizedStatementContext) {}

// ExitUnrecognizedStatement is called when production unrecognizedStatement is exited.
func (s *BaseSimplifiedCqlListener) ExitUnrecognizedStatement(ctx *UnrecognizedStatementContext) {}

// EnterUnrecognizedToken is called when production unrecognizedToken is entered.
func (s *BaseSimplifiedCqlListener) EnterUnrecognizedToken(ctx *UnrecognizedTokenContext) {}

// ExitUnrecognizedToken is called when production unrecognizedToken is exited.
func (s *BaseSimplifiedCqlListener) ExitUnrecognizedToken(ctx *UnrecognizedTokenContext) {}
