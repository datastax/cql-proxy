package parser

type Token int

const (
	TkInvalid Token = iota
	TkEOF
	TkSelect
	TkFrom
	TkUse
	TkAs
	TkCount
	TkCast
	TkIdentifier
	TkSystemIdentifier
	TkLocalIdentifier
	TkPeersIdentifier
	TkPeersV2Identifier
	TkStar
	TkComma
	TkDot
	TkLparen
	TkRparen
)

%%{
machine lex;
write data;
}%%

type Lexer struct {
	data string
	p, pe, mark int
	current string
}

func (l *Lexer) Init(data string) {
    l.p, l.pe = 0, len(data)
    l.data = data
}

func (l *Lexer) Mark() {
    l.mark = l.p
}

func (l *Lexer) Rewind() {
    l.p = l.mark
}

func (l *Lexer) Current() string {
    return l.current
}

func (l *Lexer) Next() Token {
	data := l.data
	p, pe, eof := l.p, l.pe, l.pe
	act, ts, te, cs := 0, 0, 0, -1

	token := TkInvalid

	if p == eof {
	    return TkEOF
    }

    %%{
        ws = [ \t];
        nl = '\r\n' | '\n';
        id = ([a-zA-Z][a-zA-Z0-9_]*)|("\"" ([^\r\n\"] | "\\\"")* "\"");

        main := |*
            /select/i => { token = TkSelect; fbreak; };
            /from/i => { token = TkFrom; fbreak; };
            /use/i => { token = TkUse; fbreak; };
            /as/i => { token = TkAs; fbreak; };
            /count/i => { token = TkCount; fbreak; };
            /cast/i => { token = TkCast; fbreak; };
            /system/i|'\"system\"' => { token = TkSystemIdentifier; fbreak; };
            /local/i|'\"local\"' => { token = TkLocalIdentifier; fbreak; };
            /peers/i|'\"peers\"' => { token = TkPeersIdentifier; fbreak; };
            /peers_v2/i|'\"peers_v2\"' => { token = TkPeersV2Identifier; fbreak; };
            '\*' => { token = TkStar; fbreak; };
            ',' => { token = TkComma; fbreak; };
            '\.' => { token = TkDot; fbreak; };
            '(' => { token = TkLparen; fbreak; };
            ')' => { token = TkRparen; fbreak; };
            id => { token = TkIdentifier; l.current = l.data[ts:te]; fbreak; };
            nl => { /* Skip */ };
            ws => { /* Skip */ };
            any => { token = TkInvalid; fbreak; };
        *|;

        write init;
        write exec;
    }%%

    l.p = p

    return token
}