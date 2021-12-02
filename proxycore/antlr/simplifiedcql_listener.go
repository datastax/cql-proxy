// Code generated from SimplifiedCql.g4 by ANTLR 4.7.2. DO NOT EDIT.

package parser // SimplifiedCql

import "github.com/antlr/antlr4/runtime/Go/antlr"

// SimplifiedCqlListener is a complete listener for a parse tree produced by SimplifiedCqlParser.
type SimplifiedCqlListener interface {
	antlr.ParseTreeListener

	// EnterCqlStatement is called when entering the cqlStatement production.
	EnterCqlStatement(c *CqlStatementContext)

	// EnterInsertStatement is called when entering the insertStatement production.
	EnterInsertStatement(c *InsertStatementContext)

	// EnterUpdateStatement is called when entering the updateStatement production.
	EnterUpdateStatement(c *UpdateStatementContext)

	// EnterUpdateOperations is called when entering the updateOperations production.
	EnterUpdateOperations(c *UpdateOperationsContext)

	// EnterUpdateOperation is called when entering the updateOperation production.
	EnterUpdateOperation(c *UpdateOperationContext)

	// EnterUpdateOperatorAddLeft is called when entering the updateOperatorAddLeft production.
	EnterUpdateOperatorAddLeft(c *UpdateOperatorAddLeftContext)

	// EnterUpdateOperatorAddRight is called when entering the updateOperatorAddRight production.
	EnterUpdateOperatorAddRight(c *UpdateOperatorAddRightContext)

	// EnterUpdateOperatorSubtract is called when entering the updateOperatorSubtract production.
	EnterUpdateOperatorSubtract(c *UpdateOperatorSubtractContext)

	// EnterUpdateOperatorAddAssign is called when entering the updateOperatorAddAssign production.
	EnterUpdateOperatorAddAssign(c *UpdateOperatorAddAssignContext)

	// EnterUpdateOperatorSubtractAssign is called when entering the updateOperatorSubtractAssign production.
	EnterUpdateOperatorSubtractAssign(c *UpdateOperatorSubtractAssignContext)

	// EnterDeleteStatement is called when entering the deleteStatement production.
	EnterDeleteStatement(c *DeleteStatementContext)

	// EnterDeleteOperations is called when entering the deleteOperations production.
	EnterDeleteOperations(c *DeleteOperationsContext)

	// EnterDeleteOperation is called when entering the deleteOperation production.
	EnterDeleteOperation(c *DeleteOperationContext)

	// EnterDeleteOperationElement is called when entering the deleteOperationElement production.
	EnterDeleteOperationElement(c *DeleteOperationElementContext)

	// EnterBatchStatement is called when entering the batchStatement production.
	EnterBatchStatement(c *BatchStatementContext)

	// EnterBatchChildStatement is called when entering the batchChildStatement production.
	EnterBatchChildStatement(c *BatchChildStatementContext)

	// EnterSelectStatement is called when entering the selectStatement production.
	EnterSelectStatement(c *SelectStatementContext)

	// EnterSelectClause is called when entering the selectClause production.
	EnterSelectClause(c *SelectClauseContext)

	// EnterSelectors is called when entering the selectors production.
	EnterSelectors(c *SelectorsContext)

	// EnterSelector is called when entering the selector production.
	EnterSelector(c *SelectorContext)

	// EnterUnaliasedSelector is called when entering the unaliasedSelector production.
	EnterUnaliasedSelector(c *UnaliasedSelectorContext)

	// EnterUseStatement is called when entering the useStatement production.
	EnterUseStatement(c *UseStatementContext)

	// EnterOrderByClause is called when entering the orderByClause production.
	EnterOrderByClause(c *OrderByClauseContext)

	// EnterOrderings is called when entering the orderings production.
	EnterOrderings(c *OrderingsContext)

	// EnterOrdering is called when entering the ordering production.
	EnterOrdering(c *OrderingContext)

	// EnterGroupByClause is called when entering the groupByClause production.
	EnterGroupByClause(c *GroupByClauseContext)

	// EnterPerPartitionLimitClause is called when entering the perPartitionLimitClause production.
	EnterPerPartitionLimitClause(c *PerPartitionLimitClauseContext)

	// EnterLimitClause is called when entering the limitClause production.
	EnterLimitClause(c *LimitClauseContext)

	// EnterUsingClause is called when entering the usingClause production.
	EnterUsingClause(c *UsingClauseContext)

	// EnterTimestamp is called when entering the timestamp production.
	EnterTimestamp(c *TimestampContext)

	// EnterTtl is called when entering the ttl production.
	EnterTtl(c *TtlContext)

	// EnterConditions is called when entering the conditions production.
	EnterConditions(c *ConditionsContext)

	// EnterCondition is called when entering the condition production.
	EnterCondition(c *ConditionContext)

	// EnterWhereClause is called when entering the whereClause production.
	EnterWhereClause(c *WhereClauseContext)

	// EnterRelation is called when entering the relation production.
	EnterRelation(c *RelationContext)

	// EnterOperator is called when entering the operator production.
	EnterOperator(c *OperatorContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterPrimitiveLiteral is called when entering the primitiveLiteral production.
	EnterPrimitiveLiteral(c *PrimitiveLiteralContext)

	// EnterCollectionLiteral is called when entering the collectionLiteral production.
	EnterCollectionLiteral(c *CollectionLiteralContext)

	// EnterListLiteral is called when entering the listLiteral production.
	EnterListLiteral(c *ListLiteralContext)

	// EnterSetLiteral is called when entering the setLiteral production.
	EnterSetLiteral(c *SetLiteralContext)

	// EnterMapLiteral is called when entering the mapLiteral production.
	EnterMapLiteral(c *MapLiteralContext)

	// EnterMapEntries is called when entering the mapEntries production.
	EnterMapEntries(c *MapEntriesContext)

	// EnterMapEntry is called when entering the mapEntry production.
	EnterMapEntry(c *MapEntryContext)

	// EnterTupleLiterals is called when entering the tupleLiterals production.
	EnterTupleLiterals(c *TupleLiteralsContext)

	// EnterTupleLiteral is called when entering the tupleLiteral production.
	EnterTupleLiteral(c *TupleLiteralContext)

	// EnterUdtLiteral is called when entering the udtLiteral production.
	EnterUdtLiteral(c *UdtLiteralContext)

	// EnterFieldLiterals is called when entering the fieldLiterals production.
	EnterFieldLiterals(c *FieldLiteralsContext)

	// EnterFieldLiteral is called when entering the fieldLiteral production.
	EnterFieldLiteral(c *FieldLiteralContext)

	// EnterFunctionCall is called when entering the functionCall production.
	EnterFunctionCall(c *FunctionCallContext)

	// EnterFunctionArgs is called when entering the functionArgs production.
	EnterFunctionArgs(c *FunctionArgsContext)

	// EnterFunctionArg is called when entering the functionArg production.
	EnterFunctionArg(c *FunctionArgContext)

	// EnterBindMarkers is called when entering the bindMarkers production.
	EnterBindMarkers(c *BindMarkersContext)

	// EnterBindMarker is called when entering the bindMarker production.
	EnterBindMarker(c *BindMarkerContext)

	// EnterPositionalBindMarker is called when entering the positionalBindMarker production.
	EnterPositionalBindMarker(c *PositionalBindMarkerContext)

	// EnterNamedBindMarker is called when entering the namedBindMarker production.
	EnterNamedBindMarker(c *NamedBindMarkerContext)

	// EnterTerms is called when entering the terms production.
	EnterTerms(c *TermsContext)

	// EnterTerm is called when entering the term production.
	EnterTerm(c *TermContext)

	// EnterTypeCast is called when entering the typeCast production.
	EnterTypeCast(c *TypeCastContext)

	// EnterCqlType is called when entering the cqlType production.
	EnterCqlType(c *CqlTypeContext)

	// EnterPrimitiveType is called when entering the primitiveType production.
	EnterPrimitiveType(c *PrimitiveTypeContext)

	// EnterCollectionType is called when entering the collectionType production.
	EnterCollectionType(c *CollectionTypeContext)

	// EnterTupleType is called when entering the tupleType production.
	EnterTupleType(c *TupleTypeContext)

	// EnterTableName is called when entering the tableName production.
	EnterTableName(c *TableNameContext)

	// EnterFunctionName is called when entering the functionName production.
	EnterFunctionName(c *FunctionNameContext)

	// EnterUserTypeName is called when entering the userTypeName production.
	EnterUserTypeName(c *UserTypeNameContext)

	// EnterKeyspaceName is called when entering the keyspaceName production.
	EnterKeyspaceName(c *KeyspaceNameContext)

	// EnterQualifiedIdentifier is called when entering the qualifiedIdentifier production.
	EnterQualifiedIdentifier(c *QualifiedIdentifierContext)

	// EnterIdentifiers is called when entering the identifiers production.
	EnterIdentifiers(c *IdentifiersContext)

	// EnterIdentifier is called when entering the identifier production.
	EnterIdentifier(c *IdentifierContext)

	// EnterUnreservedKeyword is called when entering the unreservedKeyword production.
	EnterUnreservedKeyword(c *UnreservedKeywordContext)

	// EnterUnrecognizedStatement is called when entering the unrecognizedStatement production.
	EnterUnrecognizedStatement(c *UnrecognizedStatementContext)

	// EnterUnrecognizedToken is called when entering the unrecognizedToken production.
	EnterUnrecognizedToken(c *UnrecognizedTokenContext)

	// ExitCqlStatement is called when exiting the cqlStatement production.
	ExitCqlStatement(c *CqlStatementContext)

	// ExitInsertStatement is called when exiting the insertStatement production.
	ExitInsertStatement(c *InsertStatementContext)

	// ExitUpdateStatement is called when exiting the updateStatement production.
	ExitUpdateStatement(c *UpdateStatementContext)

	// ExitUpdateOperations is called when exiting the updateOperations production.
	ExitUpdateOperations(c *UpdateOperationsContext)

	// ExitUpdateOperation is called when exiting the updateOperation production.
	ExitUpdateOperation(c *UpdateOperationContext)

	// ExitUpdateOperatorAddLeft is called when exiting the updateOperatorAddLeft production.
	ExitUpdateOperatorAddLeft(c *UpdateOperatorAddLeftContext)

	// ExitUpdateOperatorAddRight is called when exiting the updateOperatorAddRight production.
	ExitUpdateOperatorAddRight(c *UpdateOperatorAddRightContext)

	// ExitUpdateOperatorSubtract is called when exiting the updateOperatorSubtract production.
	ExitUpdateOperatorSubtract(c *UpdateOperatorSubtractContext)

	// ExitUpdateOperatorAddAssign is called when exiting the updateOperatorAddAssign production.
	ExitUpdateOperatorAddAssign(c *UpdateOperatorAddAssignContext)

	// ExitUpdateOperatorSubtractAssign is called when exiting the updateOperatorSubtractAssign production.
	ExitUpdateOperatorSubtractAssign(c *UpdateOperatorSubtractAssignContext)

	// ExitDeleteStatement is called when exiting the deleteStatement production.
	ExitDeleteStatement(c *DeleteStatementContext)

	// ExitDeleteOperations is called when exiting the deleteOperations production.
	ExitDeleteOperations(c *DeleteOperationsContext)

	// ExitDeleteOperation is called when exiting the deleteOperation production.
	ExitDeleteOperation(c *DeleteOperationContext)

	// ExitDeleteOperationElement is called when exiting the deleteOperationElement production.
	ExitDeleteOperationElement(c *DeleteOperationElementContext)

	// ExitBatchStatement is called when exiting the batchStatement production.
	ExitBatchStatement(c *BatchStatementContext)

	// ExitBatchChildStatement is called when exiting the batchChildStatement production.
	ExitBatchChildStatement(c *BatchChildStatementContext)

	// ExitSelectStatement is called when exiting the selectStatement production.
	ExitSelectStatement(c *SelectStatementContext)

	// ExitSelectClause is called when exiting the selectClause production.
	ExitSelectClause(c *SelectClauseContext)

	// ExitSelectors is called when exiting the selectors production.
	ExitSelectors(c *SelectorsContext)

	// ExitSelector is called when exiting the selector production.
	ExitSelector(c *SelectorContext)

	// ExitUnaliasedSelector is called when exiting the unaliasedSelector production.
	ExitUnaliasedSelector(c *UnaliasedSelectorContext)

	// ExitUseStatement is called when exiting the useStatement production.
	ExitUseStatement(c *UseStatementContext)

	// ExitOrderByClause is called when exiting the orderByClause production.
	ExitOrderByClause(c *OrderByClauseContext)

	// ExitOrderings is called when exiting the orderings production.
	ExitOrderings(c *OrderingsContext)

	// ExitOrdering is called when exiting the ordering production.
	ExitOrdering(c *OrderingContext)

	// ExitGroupByClause is called when exiting the groupByClause production.
	ExitGroupByClause(c *GroupByClauseContext)

	// ExitPerPartitionLimitClause is called when exiting the perPartitionLimitClause production.
	ExitPerPartitionLimitClause(c *PerPartitionLimitClauseContext)

	// ExitLimitClause is called when exiting the limitClause production.
	ExitLimitClause(c *LimitClauseContext)

	// ExitUsingClause is called when exiting the usingClause production.
	ExitUsingClause(c *UsingClauseContext)

	// ExitTimestamp is called when exiting the timestamp production.
	ExitTimestamp(c *TimestampContext)

	// ExitTtl is called when exiting the ttl production.
	ExitTtl(c *TtlContext)

	// ExitConditions is called when exiting the conditions production.
	ExitConditions(c *ConditionsContext)

	// ExitCondition is called when exiting the condition production.
	ExitCondition(c *ConditionContext)

	// ExitWhereClause is called when exiting the whereClause production.
	ExitWhereClause(c *WhereClauseContext)

	// ExitRelation is called when exiting the relation production.
	ExitRelation(c *RelationContext)

	// ExitOperator is called when exiting the operator production.
	ExitOperator(c *OperatorContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitPrimitiveLiteral is called when exiting the primitiveLiteral production.
	ExitPrimitiveLiteral(c *PrimitiveLiteralContext)

	// ExitCollectionLiteral is called when exiting the collectionLiteral production.
	ExitCollectionLiteral(c *CollectionLiteralContext)

	// ExitListLiteral is called when exiting the listLiteral production.
	ExitListLiteral(c *ListLiteralContext)

	// ExitSetLiteral is called when exiting the setLiteral production.
	ExitSetLiteral(c *SetLiteralContext)

	// ExitMapLiteral is called when exiting the mapLiteral production.
	ExitMapLiteral(c *MapLiteralContext)

	// ExitMapEntries is called when exiting the mapEntries production.
	ExitMapEntries(c *MapEntriesContext)

	// ExitMapEntry is called when exiting the mapEntry production.
	ExitMapEntry(c *MapEntryContext)

	// ExitTupleLiterals is called when exiting the tupleLiterals production.
	ExitTupleLiterals(c *TupleLiteralsContext)

	// ExitTupleLiteral is called when exiting the tupleLiteral production.
	ExitTupleLiteral(c *TupleLiteralContext)

	// ExitUdtLiteral is called when exiting the udtLiteral production.
	ExitUdtLiteral(c *UdtLiteralContext)

	// ExitFieldLiterals is called when exiting the fieldLiterals production.
	ExitFieldLiterals(c *FieldLiteralsContext)

	// ExitFieldLiteral is called when exiting the fieldLiteral production.
	ExitFieldLiteral(c *FieldLiteralContext)

	// ExitFunctionCall is called when exiting the functionCall production.
	ExitFunctionCall(c *FunctionCallContext)

	// ExitFunctionArgs is called when exiting the functionArgs production.
	ExitFunctionArgs(c *FunctionArgsContext)

	// ExitFunctionArg is called when exiting the functionArg production.
	ExitFunctionArg(c *FunctionArgContext)

	// ExitBindMarkers is called when exiting the bindMarkers production.
	ExitBindMarkers(c *BindMarkersContext)

	// ExitBindMarker is called when exiting the bindMarker production.
	ExitBindMarker(c *BindMarkerContext)

	// ExitPositionalBindMarker is called when exiting the positionalBindMarker production.
	ExitPositionalBindMarker(c *PositionalBindMarkerContext)

	// ExitNamedBindMarker is called when exiting the namedBindMarker production.
	ExitNamedBindMarker(c *NamedBindMarkerContext)

	// ExitTerms is called when exiting the terms production.
	ExitTerms(c *TermsContext)

	// ExitTerm is called when exiting the term production.
	ExitTerm(c *TermContext)

	// ExitTypeCast is called when exiting the typeCast production.
	ExitTypeCast(c *TypeCastContext)

	// ExitCqlType is called when exiting the cqlType production.
	ExitCqlType(c *CqlTypeContext)

	// ExitPrimitiveType is called when exiting the primitiveType production.
	ExitPrimitiveType(c *PrimitiveTypeContext)

	// ExitCollectionType is called when exiting the collectionType production.
	ExitCollectionType(c *CollectionTypeContext)

	// ExitTupleType is called when exiting the tupleType production.
	ExitTupleType(c *TupleTypeContext)

	// ExitTableName is called when exiting the tableName production.
	ExitTableName(c *TableNameContext)

	// ExitFunctionName is called when exiting the functionName production.
	ExitFunctionName(c *FunctionNameContext)

	// ExitUserTypeName is called when exiting the userTypeName production.
	ExitUserTypeName(c *UserTypeNameContext)

	// ExitKeyspaceName is called when exiting the keyspaceName production.
	ExitKeyspaceName(c *KeyspaceNameContext)

	// ExitQualifiedIdentifier is called when exiting the qualifiedIdentifier production.
	ExitQualifiedIdentifier(c *QualifiedIdentifierContext)

	// ExitIdentifiers is called when exiting the identifiers production.
	ExitIdentifiers(c *IdentifiersContext)

	// ExitIdentifier is called when exiting the identifier production.
	ExitIdentifier(c *IdentifierContext)

	// ExitUnreservedKeyword is called when exiting the unreservedKeyword production.
	ExitUnreservedKeyword(c *UnreservedKeywordContext)

	// ExitUnrecognizedStatement is called when exiting the unrecognizedStatement production.
	ExitUnrecognizedStatement(c *UnrecognizedStatementContext)

	// ExitUnrecognizedToken is called when exiting the unrecognizedToken production.
	ExitUnrecognizedToken(c *UnrecognizedTokenContext)
}
