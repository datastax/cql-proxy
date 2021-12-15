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
	tkColon
	tkQMark
	tkEqual
	tkAdd
	tkSub
	tkAddEqual
	tkSubEqual
	tkLparen
	tkRparen
	tkLsquare
	tkRsquare
	tkLcurly
	tkRcurly
	tkInteger
	tkFloat
	tkBool
	tkStringLiteral
	tkHexNumber
	tkUuid
	tkDuration
	tkNan
	tkInfinity
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
        integer = '-'? digit+;
        exponent = [eE] ('+' | '-')? digit+;
        float = (integer exponent) | (integer '.' digit* exponent?);
        string = '\'' ([^\'] | '\'\'') '\'';
        pgstring = '$' ([^\$] | '$$') '$';
        hex = [a-f] | [A-F] | digit;
        hexnumber = '0' [xX] hex*;
        uuid = hex{8} '-' hex{4} '-' hex{4} '-' hex{4} '-' hex{12};
        durationunit = /y/i | /mo/i | /w/i | /d/i | /h/i | /m/i | /s/i | /ms/i | /µs/i | /us/i | /ns/i;
        duration = ('-'? digit+ durationunit (digit+ durationunit)*) |
                   ('-'? 'P' (digit+ 'Y')? (digit+ 'M')? (digit+ 'D')? ('T' (digit+ 'H')? (digit+ 'M')? (digit+ 'S')?)?) |
                   ('-'? 'P' digit+ 'W') |
                   '-'? 'P' digit digit digit digit '-' digit digit '-' digit digit 'T' digit digit ':' digit digit ':' digit digit;
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
            /true/i | /false/i => { tk = tkBool; fbreak; };
            '\*' => { tk = tkStar; fbreak; };
            ',' => { tk = tkComma; fbreak; };
            '\.' => { tk = tkDot; fbreak; };
            ':' => { tk = tkColon; fbreak; };
            '?' => { tk = tkQMark; fbreak; };
            '(' => { tk = tkLparen; fbreak; };
            ')' => { tk = tkRparen; fbreak; };
            '[' => { tk = tkLsquare; fbreak; };
            ']' => { tk = tkRsquare; fbreak; };
            '{' => { tk = tkLcurly; fbreak; };
            '}' => { tk = tkRcurly; fbreak; };
            '=' => { tk = tkEqual; fbreak; };
            '+' => { tk = tkAdd; fbreak; };
            '-' => { tk = tkSub; fbreak; };
            '+=' => { tk = tkAddEqual; fbreak; };
            '-=' => { tk = tkSubEqual; fbreak; };
            '-'? /nan/i => { tk = tkNan; fbreak; };
            '-'? /infinity/i => { tk = tkInfinity; fbreak; };
            pgstring | string => { tk = tkStringLiteral; fbreak; };
            integer => { tk = tkInteger; fbreak; };
            float => { tk = tkFloat; fbreak; };
            hexnumber => { tk = tkHexNumber; fbreak; };
            duration => { tk = tkDuration; fbreak; };
            uuid => { tk = tkUuid; fbreak; };
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