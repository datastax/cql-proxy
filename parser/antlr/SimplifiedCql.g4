// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

grammar SimplifiedCql;

// PARSER

cqlStatement
    : insertStatement EOS?
    | updateStatement EOS?
    | deleteStatement EOS?
    | batchStatement EOS?
    | selectStatement EOS?
    | useStatement EOS?
    | unrecognizedStatement EOS?
    ;

// INSERT

// note: JSON INSERT not supported
insertStatement
    : K_INSERT K_INTO tableName
      '(' identifiers ')' K_VALUES '(' insertTerms ')'
      ( K_IF K_NOT K_EXISTS )?
      usingClause?
    ;

insertTerms
    : terms
    ;

// UPDATE

updateStatement
    : K_UPDATE tableName
      usingClause?
      K_SET updateOperations
      whereClause
      ( K_IF ( K_EXISTS | conditions ))?
    ;

updateOperations
    : updateOperation ( ',' updateOperation )*
    ;

updateOperation
    : identifier '=' term
    | updateOperatorAddLeft
    | updateOperatorAddRight
    | updateOperatorSubtract
    | updateOperatorAddAssign
    | updateOperatorSubtractAssign
    | identifier '[' term ']' '=' term
    | identifier '.' identifier '=' term
    ;

updateOperatorAddLeft
    : identifier '=' term '+' identifier
    ;

updateOperatorAddRight
    : identifier '=' identifier '+' term
    ;

updateOperatorSubtract
    : identifier '=' identifier '-' term
    ;

updateOperatorAddAssign
    : identifier '+=' term
    ;

updateOperatorSubtractAssign
    : identifier '-=' term
    ;

// DELETE

deleteStatement
    : K_DELETE  deleteOperations?
      K_FROM tableName
      ( K_USING timestamp )?
      whereClause
      ( K_IF ( K_EXISTS | conditions ) )?
    ;

deleteOperations
    : deleteOperation ( ',' deleteOperation )*
    ;

deleteOperation
    : identifier
    | deleteOperationElement
    | identifier '.' identifier
    ;

deleteOperationElement
    : identifier '[' term ']'
    ;

// BATCH

batchStatement
    : K_BEGIN ( K_UNLOGGED | K_COUNTER )? K_BATCH
      usingClause?
      ( batchChildStatement EOS? )*
      K_APPLY K_BATCH
    ;

batchChildStatement
    : insertStatement
    | updateStatement
    | deleteStatement
    ;

// SELECT

selectStatement
    : K_SELECT K_JSON? K_DISTINCT?
      selectClause
      K_FROM tableName
      whereClause?
      groupByClause?
      orderByClause?
      perPartitionLimitClause?
      limitClause?
      ( K_ALLOW K_FILTERING )?
    ;

selectClause
    : '*'
    | selectors
    ;

selectors
    : selector ( ',' selector )*
    ;

selector
    : unaliasedSelector ( K_AS identifier )?
    ;

unaliasedSelector
    : identifier
    | term
    | K_COUNT '(' '*' ')'
    | K_CAST '(' unaliasedSelector K_AS primitiveType ')'
    ;

// USE

useStatement
    : K_USE keyspaceName
    ;

// CLAUSES

// ORDER BY, GROUP BY, LIMIT

orderByClause
    : K_ORDER K_BY orderings
    ;

orderings
    : ordering ( ',' ordering )*
    ;

ordering
    : identifier ( K_ASC | K_DESC )?
    ;

groupByClause
    : K_GROUP K_BY identifiers
    ;

perPartitionLimitClause
    : K_PER K_PARTITION K_LIMIT ( INTEGER | bindMarker )
    ;

limitClause
    : K_LIMIT ( INTEGER | bindMarker )
    ;

// USING

usingClause
    : K_USING timestamp
    | K_USING ttl
    | K_USING timestamp K_AND ttl
    | K_USING ttl K_AND timestamp
    ;

timestamp
    : K_TIMESTAMP ( INTEGER | bindMarker )
    ;

ttl
    : K_TTL ( INTEGER | bindMarker )
    ;

// LWT

conditions
    : condition ( K_AND condition )*
    ;

condition
    : identifier operator term
    | identifier K_IN ( '(' terms? ')' | bindMarker )
    | identifier '[' term ']' operator term
    | identifier '[' term ']' K_IN ( '(' terms? ')' | bindMarker )
    | identifier '.' identifier operator term
    | identifier '.' identifier K_IN ( '(' terms? ')' | bindMarker )
    ;

// WHERE

// Note: custom index expressions not supported
whereClause
    : K_WHERE relation ( K_AND relation )*
    ;

relation
    : identifier operator term
    | K_TOKEN '(' identifiers ')' operator term
    | identifier K_LIKE term
    | identifier K_IS K_NOT K_NULL
    | identifier K_CONTAINS K_KEY? term
    | identifier '[' term ']' operator term
    | identifier K_IN ( '(' terms? ')' | bindMarker )
    | '(' identifiers ')' K_IN '(' ')'
    | '(' identifiers ')' K_IN bindMarker              // (a, b, c) IN ?
    | '(' identifiers ')' K_IN '(' tupleLiterals ')'   // (a, b, c) IN ((1, 2, 3), (4, 5, 6), ...)
    | '(' identifiers ')' K_IN '(' bindMarkers ')'     // (a, b, c) IN (?, ?, ...)
    | '(' identifiers ')' operator tupleLiteral        // (a, b, c) > (1, 2, 3)
    | '(' identifiers ')' operator '(' bindMarkers ')' // (a, b, c) > (?, ?, ?)
    | '(' identifiers ')' operator bindMarker          // (a, b, c) >= ?
    | '(' relation ')'
    ;

operator
    : '='
    | '<'
    | '<='
    | '>'
    | '>='
    | '!='
    ;

// CQL literals

literal
    : primitiveLiteral
    | collectionLiteral
    | tupleLiteral
    | udtLiteral
    | K_NULL
    ;

primitiveLiteral
    : STRING_LITERAL
    | INTEGER
    | FLOAT
    | BOOLEAN
    | DURATION
    | UUID
    | HEXNUMBER
    | '-'? K_NAN
    | '-'? K_INFINITY
    ;

collectionLiteral
    : listLiteral
    | setLiteral
    | mapLiteral
    ;

listLiteral
    : '[' terms? ']'
    ;

setLiteral
    : '{' terms? '}'
    ;

mapLiteral
    : '{' mapEntries? '}'
    ;

mapEntries
    : mapEntry ( ',' mapEntry )*
    ;

mapEntry
    : term ':' term
    ;

tupleLiterals
    : tupleLiteral ( ',' tupleLiteral )*
    ;

tupleLiteral
    : '(' terms ')'
    ;

// Note: empty user types literals not allowed
udtLiteral
    : '{' fieldLiterals '}'
    ;

fieldLiterals
    : fieldLiteral ( ',' fieldLiteral )*
    ;

fieldLiteral
    : identifier ':' term
    ;

// Functions, bind markers and terms

functionCall
    : functionName '(' functionArgs? ')'
    ;

functionArgs
    : functionArg ( ',' functionArg )*
    ;

functionArg
    : identifier
    | term
    ;

bindMarkers
    : bindMarker ( ',' bindMarker )*
    ;

bindMarker
    : positionalBindMarker
    | namedBindMarker
    ;

positionalBindMarker
    : QMARK
    ;

namedBindMarker
    : ':' identifier
    ;

terms
    : term ( ',' term )*
    ;

term
    : literal
    | bindMarker
    | functionCall
    | typeCast
    ;

typeCast
    : '(' cqlType ')' term
    ;

// CQL types

cqlType
    : primitiveType
    | collectionType
    | tupleType
    | userTypeName
    | K_FROZEN '<' cqlType '>'
    ;

primitiveType
    : K_ASCII
    | K_BIGINT
    | K_BLOB
    | K_BOOLEAN
    | K_COUNTER
    | K_DATE
    | K_DECIMAL
    | K_DOUBLE
    | K_DURATION
    | K_FLOAT
    | K_INET
    | K_INT
    | K_SMALLINT
    | K_TEXT
    | K_TIME
    | K_TIMESTAMP
    | K_TIMEUUID
    | K_TINYINT
    | K_UUID
    | K_VARCHAR
    | K_VARINT
    ;

collectionType
    : K_LIST '<' cqlType '>'
    | K_SET  '<' cqlType '>'
    | K_MAP  '<' cqlType ',' cqlType '>'
    ;

tupleType
    : K_TUPLE '<' cqlType (',' cqlType)* '>'
    ;

// identifiers

tableName
    : qualifiedIdentifier
    ;

functionName
    : qualifiedIdentifier
    ;

userTypeName
    : qualifiedIdentifier
    ;

keyspaceName
    : identifier
    ;

qualifiedIdentifier
    : ( keyspaceName '.' )? identifier
    ;

identifiers
    : identifier ( ',' identifier )*
    ;

identifier
    : UNQUOTED_IDENTIFIER
    | QUOTED_IDENTIFIER
    | unreservedKeyword
    ;

// Unreserved keywords

unreservedKeyword
    : K_AS
    | K_CAST
    | K_CLUSTERING
    | K_CONTAINS
    | K_COUNT
    | K_DISTINCT
    | K_EXISTS
    | K_FILTERING
    | K_FROZEN
    | K_GROUP
    | K_JSON
    | K_KEY
    | K_LIKE
    | K_LIST
    | K_MAP
    | K_PARTITION
    | K_PER
    | K_TTL
    | K_TUPLE
    | K_TYPE
    | K_VALUES
    | K_WRITETIME
    | primitiveType
    ;

// Unrecognized statements

unrecognizedStatement
    : unrecognizedToken*
    ;

unrecognizedToken
    : .
    ;

// LEXER

K_ALLOW:       A L L O W;
K_AND:         A N D;
K_APPLY:       A P P L Y;
K_ASC:         A S C;
K_AS:          A S;
K_ASCII:       A S C I I;
K_BATCH:       B A T C H;
K_BEGIN:       B E G I N;
K_BIGINT:      B I G I N T;
K_BLOB:        B L O B;
K_BOOLEAN:     B O O L E A N;
K_BY:          B Y;
K_CAST:        C A S T;
K_CLUSTERING:  C L U S T E R I N G;
K_CONTAINS:    C O N T A I N S;
K_COUNTER:     C O U N T E R;
K_COUNT:       C O U N T;
K_DATE:        D A T E;
K_DECIMAL:     D E C I M A L;
K_DELETE:      D E L E T E;
K_DESC:        D E S C;
K_DISTINCT:    D I S T I N C T;
K_DOUBLE:      D O U B L E;
K_DURATION:    D U R A T I O N;
K_EXISTS:      E X I S T S;
K_FILTERING:   F I L T E R I N G;
K_FLOAT:       F L O A T;
K_FROM:        F R O M;
K_FROZEN:      F R O Z E N;
K_GROUP:       G R O U P;
K_IF:          I F;
K_INET:        I N E T;
K_INFINITY:    I N F I N I T Y;
K_INSERT:      I N S E R T;
K_INTO:        I N T O;
K_INT:         I N T;
K_IN:          I N;
K_IS:          I S;
K_JSON:        J S O N;
K_KEY:         K E Y;
K_LIKE:        L I K E;
K_LIMIT:       L I M I T;
K_LIST:        L I S T;
K_MAP:         M A P;
K_NAN:         N A N;
K_NOT:         N O T;
K_NULL:        N U L L;
K_ORDER:       O R D E R;
K_PARTITION:   P A R T I T I O N;
K_PER:         P E R;
K_SELECT:      S E L E C T;
K_SET:         S E T;
K_SMALLINT:    S M A L L I N T;
K_TEXT:        T E X T;
K_TIMESTAMP:   T I M E S T A M P;
K_TIMEUUID:    T I M E U U I D;
K_TIME:        T I M E;
K_TINYINT:     T I N Y I N T;
K_TOKEN:       T O K E N;
K_TTL:         T T L;
K_TUPLE:       T U P L E;
K_TYPE:        T Y P E;
K_UNLOGGED:    U N L O G G E D;
K_UPDATE:      U P D A T E;
K_USE:         U S E;
K_USING:       U S I N G;
K_UUID:        U U I D;
K_VALUES:      V A L U E S;
K_VARCHAR:     V A R C H A R;
K_VARINT:      V A R I N T;
K_WHERE:       W H E R E;
K_WRITETIME:   W R I T E T I M E;

// Case-insensitive alpha characters
fragment A: ('a'|'A');
fragment B: ('b'|'B');
fragment C: ('c'|'C');
fragment D: ('d'|'D');
fragment E: ('e'|'E');
fragment F: ('f'|'F');
fragment G: ('g'|'G');
fragment H: ('h'|'H');
fragment I: ('i'|'I');
fragment J: ('j'|'J');
fragment K: ('k'|'K');
fragment L: ('l'|'L');
fragment M: ('m'|'M');
fragment N: ('n'|'N');
fragment O: ('o'|'O');
fragment P: ('p'|'P');
fragment Q: ('q'|'Q');
fragment R: ('r'|'R');
fragment S: ('s'|'S');
fragment T: ('t'|'T');
fragment U: ('u'|'U');
fragment V: ('v'|'V');
fragment W: ('w'|'W');
fragment X: ('x'|'X');
fragment Y: ('y'|'Y');
fragment Z: ('z'|'Z');

STRING_LITERAL
    : /* pg-style string literal */
      '$' '$' ( ~'$' | '$' ~'$' )* '$' '$'
    | /* conventional quoted string literal */
      '\'' ( ~'\'' | '\'' '\'' )* '\''
    ;

QUOTED_IDENTIFIER
    : '"' ( ~'"' | '"' '"' )+ '"'
    ;

fragment DIGIT
    : '0'..'9'
    ;

fragment LETTER
    : ('A'..'Z' | 'a'..'z')
    ;

fragment HEX
    : ('A'..'F' | 'a'..'f' | '0'..'9')
    ;

fragment EXPONENT
    : E ('+' | '-')? DIGIT+
    ;

fragment DURATION_UNIT
    : Y
    | M O
    | W
    | D
    | H
    | M
    | S
    | M S
    | U S
    | '\u00B5' S
    | N S
    ;

INTEGER
    : '-'? DIGIT+
    ;

QMARK
    : '?'
    ;

FLOAT
    : INTEGER EXPONENT
    | INTEGER '.' DIGIT* EXPONENT?
    ;

/*
 * This has to be before UNQUOTED_IDENTIFIER so it takes precendence over it.
 */
BOOLEAN
    : T R U E | F A L S E
    ;

DURATION
    : '-'? DIGIT+ DURATION_UNIT (DIGIT+ DURATION_UNIT)*
    | '-'? 'P' (DIGIT+ 'Y')? (DIGIT+ 'M')? (DIGIT+ 'D')? ('T' (DIGIT+ 'H')? (DIGIT+ 'M')? (DIGIT+ 'S')?)? // ISO 8601 "format with designators"
    | '-'? 'P' DIGIT+ 'W'
    | '-'? 'P' DIGIT DIGIT DIGIT DIGIT '-' DIGIT DIGIT '-' DIGIT DIGIT 'T' DIGIT DIGIT ':' DIGIT DIGIT ':' DIGIT DIGIT // ISO 8601 "alternative format"
    ;

UNQUOTED_IDENTIFIER
    : LETTER (LETTER | DIGIT | '_')*
    ;

HEXNUMBER
    : '0' X HEX*
    ;

UUID
    : HEX HEX HEX HEX HEX HEX HEX HEX '-'
      HEX HEX HEX HEX '-'
      HEX HEX HEX HEX '-'
      HEX HEX HEX HEX '-'
      HEX HEX HEX HEX HEX HEX HEX HEX HEX HEX HEX HEX
    ;

WS
    : (' ' | '\t' | '\n' | '\r')+ -> channel(HIDDEN)
    ;

COMMENT
    : ('--' | '//') .*? ('\n'|'\r') -> channel(HIDDEN)
    ;

MULTILINE_COMMENT
    : '/*' .*? '*/' -> channel(HIDDEN)
    ;

// End of statement
EOS
    : ';'
    ;

// A catch-all lexer rule to catch everything that wasn't recognized and avoid "token recognition" warnings.
OTHER
    : .
    ;
