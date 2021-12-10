package parser

type token int

const (
	tkInvalid token = iota
	tkEOF
	tkSelect
	tkInsert
	tkUpdate
	tkDelete
	tkBegin
	tkApply
	tkBatch
	tkInto
	tkValues
	tkSet
	tkFrom
	tkUse
	tkIf
	tkAs
	tkCount
	tkCast
	tkIdentifier
	tkStar
	tkComma
	tkDot
	tkLparen
	tkRparen
)

%%{
machine lex;
write data;
}%%

type lexer struct {
	data string
	p, pe, m int
	c string
}

func (l *lexer) init(data string) {
    l.p, l.pe = 0, len(data)
    l.data = data
}

func (l *lexer) mark() {
    l.m = l.p
}

func (l *lexer) rewind() {
    l.p = l.m
}

func (l *lexer) current() string {
    return l.c
}

func (l *lexer) next() token {
	data := l.data
	p, pe, eof := l.p, l.pe, l.pe
	act, ts, te, cs := 0, 0, 0, -1

	tk := tkInvalid

	if p == eof {
	    return tkEOF
    }

    %%{
        ws = [ \t];
        nl = '\r\n' | '\n';
        id = ([a-zA-Z][a-zA-Z0-9_]*)|("\"" ([^\r\n\"] | "\\\"")* "\"");

        main := |*
            /select/i => { tk = tkSelect; fbreak; };
            /insert/i => { tk = tkInsert; fbreak; };
            /update/i => { tk = tkUpdate; fbreak; };
            /delete/i => { tk = tkDelete; fbreak; };
            /batch/i => { tk = tkBatch; fbreak; };
            /begin/i => { tk = tkBegin; fbreak; };
            /apply/i => { tk = tkApply; fbreak; };
            /into/i => { tk = tkInto; fbreak; };
            /values/i => { tk = tkValues; fbreak; };
            /set/i => { tk = tkSet; fbreak; };
            /from/i => { tk = tkFrom; fbreak; };
            /use/i => { tk = tkUse; fbreak; };
            /if/i => { tk = tkIf; fbreak; };
            /as/i => { tk = tkAs; fbreak; };
            /count/i => { tk = tkCount; fbreak; };
            /cast/i => { tk = tkCast; fbreak; };
            '\*' => { tk = tkStar; fbreak; };
            ',' => { tk = tkComma; fbreak; };
            '\.' => { tk = tkDot; fbreak; };
            '(' => { tk = tkLparen; fbreak; };
            ')' => { tk = tkRparen; fbreak; };
            id => { tk = tkIdentifier; l.c = l.data[ts:te]; fbreak; };
            nl => { /* Skip */ };
            ws => { /* Skip */ };
            any => { tk = tkInvalid; fbreak; };
        *|;

        write init;
        write exec;
    }%%

    l.p = p

    return tk
}