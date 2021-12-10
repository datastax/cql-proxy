
//line lexer.rl:1
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


//line lexer.go:30
const lex_start int = 21
const lex_first_final int = 21
const lex_error int = -1

const lex_en_main int = 21


//line lexer.rl:29


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

    
//line lexer.go:76
	{
	cs = lex_start
	ts = 0
	te = 0
	act = 0
	}

//line lexer.go:84
	{
	if p == pe {
		goto _test_eof
	}
	switch cs {
	case 21:
		goto st_case_21
	case 22:
		goto st_case_22
	case 23:
		goto st_case_23
	case 0:
		goto st_case_0
	case 1:
		goto st_case_1
	case 24:
		goto st_case_24
	case 2:
		goto st_case_2
	case 3:
		goto st_case_3
	case 4:
		goto st_case_4
	case 5:
		goto st_case_5
	case 6:
		goto st_case_6
	case 7:
		goto st_case_7
	case 8:
		goto st_case_8
	case 9:
		goto st_case_9
	case 10:
		goto st_case_10
	case 11:
		goto st_case_11
	case 12:
		goto st_case_12
	case 13:
		goto st_case_13
	case 14:
		goto st_case_14
	case 15:
		goto st_case_15
	case 16:
		goto st_case_16
	case 17:
		goto st_case_17
	case 18:
		goto st_case_18
	case 19:
		goto st_case_19
	case 20:
		goto st_case_20
	case 25:
		goto st_case_25
	case 26:
		goto st_case_26
	case 27:
		goto st_case_27
	case 28:
		goto st_case_28
	case 29:
		goto st_case_29
	case 30:
		goto st_case_30
	case 31:
		goto st_case_31
	case 32:
		goto st_case_32
	case 33:
		goto st_case_33
	case 34:
		goto st_case_34
	case 35:
		goto st_case_35
	case 36:
		goto st_case_36
	case 37:
		goto st_case_37
	case 38:
		goto st_case_38
	case 39:
		goto st_case_39
	case 40:
		goto st_case_40
	case 41:
		goto st_case_41
	case 42:
		goto st_case_42
	case 43:
		goto st_case_43
	case 44:
		goto st_case_44
	case 45:
		goto st_case_45
	case 46:
		goto st_case_46
	case 47:
		goto st_case_47
	case 48:
		goto st_case_48
	case 49:
		goto st_case_49
	case 50:
		goto st_case_50
	case 51:
		goto st_case_51
	case 52:
		goto st_case_52
	case 53:
		goto st_case_53
	case 54:
		goto st_case_54
	case 55:
		goto st_case_55
	case 56:
		goto st_case_56
	case 57:
		goto st_case_57
	}
	goto st_out
tr0:
//line NONE:1
	switch act {
	case 1:
	{p = (te) - 1
 token = TkSelect; {p++; cs = 21; goto _out } }
	case 2:
	{p = (te) - 1
 token = TkFrom; {p++; cs = 21; goto _out } }
	case 3:
	{p = (te) - 1
 token = TkUse; {p++; cs = 21; goto _out } }
	case 4:
	{p = (te) - 1
 token = TkAs; {p++; cs = 21; goto _out } }
	case 5:
	{p = (te) - 1
 token = TkCount; {p++; cs = 21; goto _out } }
	case 6:
	{p = (te) - 1
 token = TkCast; {p++; cs = 21; goto _out } }
	case 7:
	{p = (te) - 1
 token = TkSystemIdentifier; {p++; cs = 21; goto _out } }
	case 8:
	{p = (te) - 1
 token = TkLocalIdentifier; {p++; cs = 21; goto _out } }
	case 10:
	{p = (te) - 1
 token = TkPeersV2Identifier; {p++; cs = 21; goto _out } }
	case 16:
	{p = (te) - 1
 token = TkIdentifier; l.current = l.data[ts:te]; {p++; cs = 21; goto _out } }
	case 19:
	{p = (te) - 1
 token = TkInvalid; {p++; cs = 21; goto _out } }
	}
	
	goto st21
tr2:
//line lexer.rl:86
te = p+1
{ token = TkIdentifier; l.current = l.data[ts:te]; {p++; cs = 21; goto _out } }
	goto st21
tr5:
//line lexer.rl:89
p = (te) - 1
{ token = TkInvalid; {p++; cs = 21; goto _out } }
	goto st21
tr10:
//line lexer.rl:78
te = p+1
{ token = TkLocalIdentifier; {p++; cs = 21; goto _out } }
	goto st21
tr15:
//line lexer.rl:79
te = p+1
{ token = TkPeersIdentifier; {p++; cs = 21; goto _out } }
	goto st21
tr19:
//line lexer.rl:80
te = p+1
{ token = TkPeersV2Identifier; {p++; cs = 21; goto _out } }
	goto st21
tr25:
//line lexer.rl:77
te = p+1
{ token = TkSystemIdentifier; {p++; cs = 21; goto _out } }
	goto st21
tr26:
//line lexer.rl:89
te = p+1
{ token = TkInvalid; {p++; cs = 21; goto _out } }
	goto st21
tr27:
//line lexer.rl:88
te = p+1
{ /* Skip */ }
	goto st21
tr28:
//line lexer.rl:87
te = p+1
{ /* Skip */ }
	goto st21
tr31:
//line lexer.rl:84
te = p+1
{ token = TkLparen; {p++; cs = 21; goto _out } }
	goto st21
tr32:
//line lexer.rl:85
te = p+1
{ token = TkRparen; {p++; cs = 21; goto _out } }
	goto st21
tr33:
//line lexer.rl:81
te = p+1
{ token = TkStar; {p++; cs = 21; goto _out } }
	goto st21
tr34:
//line lexer.rl:82
te = p+1
{ token = TkComma; {p++; cs = 21; goto _out } }
	goto st21
tr35:
//line lexer.rl:83
te = p+1
{ token = TkDot; {p++; cs = 21; goto _out } }
	goto st21
tr44:
//line lexer.rl:89
te = p
p--
{ token = TkInvalid; {p++; cs = 21; goto _out } }
	goto st21
tr48:
//line lexer.rl:86
te = p
p--
{ token = TkIdentifier; l.current = l.data[ts:te]; {p++; cs = 21; goto _out } }
	goto st21
tr68:
//line lexer.rl:79
te = p
p--
{ token = TkPeersIdentifier; {p++; cs = 21; goto _out } }
	goto st21
	st21:
//line NONE:1
ts = 0

		if p++; p == pe {
			goto _test_eof21
		}
	st_case_21:
//line NONE:1
ts = p

//line lexer.go:346
		switch data[p] {
		case 9:
			goto tr27
		case 10:
			goto tr28
		case 13:
			goto st22
		case 32:
			goto tr27
		case 34:
			goto tr30
		case 40:
			goto tr31
		case 41:
			goto tr32
		case 42:
			goto tr33
		case 44:
			goto tr34
		case 46:
			goto tr35
		case 65:
			goto st25
		case 67:
			goto st27
		case 70:
			goto st33
		case 76:
			goto st36
		case 80:
			goto st40
		case 83:
			goto st47
		case 85:
			goto st56
		case 97:
			goto st25
		case 99:
			goto st27
		case 102:
			goto st33
		case 108:
			goto st36
		case 112:
			goto st40
		case 115:
			goto st47
		case 117:
			goto st56
		}
		switch {
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		case data[p] >= 66:
			goto tr37
		}
		goto tr26
	st22:
		if p++; p == pe {
			goto _test_eof22
		}
	st_case_22:
		if data[p] == 10 {
			goto tr28
		}
		goto tr44
tr30:
//line NONE:1
te = p+1

//line lexer.rl:89
act = 19;
	goto st23
	st23:
		if p++; p == pe {
			goto _test_eof23
		}
	st_case_23:
//line lexer.go:427
		switch data[p] {
		case 10:
			goto tr44
		case 13:
			goto tr44
		case 34:
			goto tr2
		case 92:
			goto st1
		case 108:
			goto st2
		case 112:
			goto st7
		case 115:
			goto st15
		}
		goto st0
	st0:
		if p++; p == pe {
			goto _test_eof0
		}
	st_case_0:
		switch data[p] {
		case 10:
			goto tr0
		case 13:
			goto tr0
		case 34:
			goto tr2
		case 92:
			goto st1
		}
		goto st0
	st1:
		if p++; p == pe {
			goto _test_eof1
		}
	st_case_1:
		switch data[p] {
		case 10:
			goto tr0
		case 13:
			goto tr0
		case 34:
			goto tr4
		case 92:
			goto st1
		}
		goto st0
tr4:
//line NONE:1
te = p+1

//line lexer.rl:86
act = 16;
	goto st24
	st24:
		if p++; p == pe {
			goto _test_eof24
		}
	st_case_24:
//line lexer.go:489
		switch data[p] {
		case 10:
			goto tr48
		case 13:
			goto tr48
		case 34:
			goto tr2
		case 92:
			goto st1
		}
		goto st0
	st2:
		if p++; p == pe {
			goto _test_eof2
		}
	st_case_2:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 111:
			goto st3
		}
		goto st0
	st3:
		if p++; p == pe {
			goto _test_eof3
		}
	st_case_3:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 99:
			goto st4
		}
		goto st0
	st4:
		if p++; p == pe {
			goto _test_eof4
		}
	st_case_4:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 97:
			goto st5
		}
		goto st0
	st5:
		if p++; p == pe {
			goto _test_eof5
		}
	st_case_5:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 108:
			goto st6
		}
		goto st0
	st6:
		if p++; p == pe {
			goto _test_eof6
		}
	st_case_6:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr10
		case 92:
			goto st1
		}
		goto st0
	st7:
		if p++; p == pe {
			goto _test_eof7
		}
	st_case_7:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 101:
			goto st8
		}
		goto st0
	st8:
		if p++; p == pe {
			goto _test_eof8
		}
	st_case_8:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 101:
			goto st9
		}
		goto st0
	st9:
		if p++; p == pe {
			goto _test_eof9
		}
	st_case_9:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 114:
			goto st10
		}
		goto st0
	st10:
		if p++; p == pe {
			goto _test_eof10
		}
	st_case_10:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 115:
			goto st11
		}
		goto st0
	st11:
		if p++; p == pe {
			goto _test_eof11
		}
	st_case_11:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr15
		case 92:
			goto st1
		case 95:
			goto st12
		}
		goto st0
	st12:
		if p++; p == pe {
			goto _test_eof12
		}
	st_case_12:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 118:
			goto st13
		}
		goto st0
	st13:
		if p++; p == pe {
			goto _test_eof13
		}
	st_case_13:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 50:
			goto st14
		case 92:
			goto st1
		}
		goto st0
	st14:
		if p++; p == pe {
			goto _test_eof14
		}
	st_case_14:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr19
		case 92:
			goto st1
		}
		goto st0
	st15:
		if p++; p == pe {
			goto _test_eof15
		}
	st_case_15:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 121:
			goto st16
		}
		goto st0
	st16:
		if p++; p == pe {
			goto _test_eof16
		}
	st_case_16:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 115:
			goto st17
		}
		goto st0
	st17:
		if p++; p == pe {
			goto _test_eof17
		}
	st_case_17:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 116:
			goto st18
		}
		goto st0
	st18:
		if p++; p == pe {
			goto _test_eof18
		}
	st_case_18:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 101:
			goto st19
		}
		goto st0
	st19:
		if p++; p == pe {
			goto _test_eof19
		}
	st_case_19:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr2
		case 92:
			goto st1
		case 109:
			goto st20
		}
		goto st0
	st20:
		if p++; p == pe {
			goto _test_eof20
		}
	st_case_20:
		switch data[p] {
		case 10:
			goto tr5
		case 13:
			goto tr5
		case 34:
			goto tr25
		case 92:
			goto st1
		}
		goto st0
	st25:
		if p++; p == pe {
			goto _test_eof25
		}
	st_case_25:
		switch data[p] {
		case 83:
			goto tr49
		case 95:
			goto tr37
		case 115:
			goto tr49
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
tr37:
//line NONE:1
te = p+1

//line lexer.rl:86
act = 16;
	goto st26
tr49:
//line NONE:1
te = p+1

//line lexer.rl:74
act = 4;
	goto st26
tr53:
//line NONE:1
te = p+1

//line lexer.rl:76
act = 6;
	goto st26
tr56:
//line NONE:1
te = p+1

//line lexer.rl:75
act = 5;
	goto st26
tr59:
//line NONE:1
te = p+1

//line lexer.rl:72
act = 2;
	goto st26
tr63:
//line NONE:1
te = p+1

//line lexer.rl:78
act = 8;
	goto st26
tr71:
//line NONE:1
te = p+1

//line lexer.rl:80
act = 10;
	goto st26
tr77:
//line NONE:1
te = p+1

//line lexer.rl:71
act = 1;
	goto st26
tr81:
//line NONE:1
te = p+1

//line lexer.rl:77
act = 7;
	goto st26
tr83:
//line NONE:1
te = p+1

//line lexer.rl:73
act = 3;
	goto st26
	st26:
		if p++; p == pe {
			goto _test_eof26
		}
	st_case_26:
//line lexer.go:938
		if data[p] == 95 {
			goto tr37
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr0
	st27:
		if p++; p == pe {
			goto _test_eof27
		}
	st_case_27:
		switch data[p] {
		case 65:
			goto st28
		case 79:
			goto st30
		case 95:
			goto tr37
		case 97:
			goto st28
		case 111:
			goto st30
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st28:
		if p++; p == pe {
			goto _test_eof28
		}
	st_case_28:
		switch data[p] {
		case 83:
			goto st29
		case 95:
			goto tr37
		case 115:
			goto st29
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st29:
		if p++; p == pe {
			goto _test_eof29
		}
	st_case_29:
		switch data[p] {
		case 84:
			goto tr53
		case 95:
			goto tr37
		case 116:
			goto tr53
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st30:
		if p++; p == pe {
			goto _test_eof30
		}
	st_case_30:
		switch data[p] {
		case 85:
			goto st31
		case 95:
			goto tr37
		case 117:
			goto st31
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st31:
		if p++; p == pe {
			goto _test_eof31
		}
	st_case_31:
		switch data[p] {
		case 78:
			goto st32
		case 95:
			goto tr37
		case 110:
			goto st32
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st32:
		if p++; p == pe {
			goto _test_eof32
		}
	st_case_32:
		switch data[p] {
		case 84:
			goto tr56
		case 95:
			goto tr37
		case 116:
			goto tr56
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st33:
		if p++; p == pe {
			goto _test_eof33
		}
	st_case_33:
		switch data[p] {
		case 82:
			goto st34
		case 95:
			goto tr37
		case 114:
			goto st34
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st34:
		if p++; p == pe {
			goto _test_eof34
		}
	st_case_34:
		switch data[p] {
		case 79:
			goto st35
		case 95:
			goto tr37
		case 111:
			goto st35
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st35:
		if p++; p == pe {
			goto _test_eof35
		}
	st_case_35:
		switch data[p] {
		case 77:
			goto tr59
		case 95:
			goto tr37
		case 109:
			goto tr59
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st36:
		if p++; p == pe {
			goto _test_eof36
		}
	st_case_36:
		switch data[p] {
		case 79:
			goto st37
		case 95:
			goto tr37
		case 111:
			goto st37
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st37:
		if p++; p == pe {
			goto _test_eof37
		}
	st_case_37:
		switch data[p] {
		case 67:
			goto st38
		case 95:
			goto tr37
		case 99:
			goto st38
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st38:
		if p++; p == pe {
			goto _test_eof38
		}
	st_case_38:
		switch data[p] {
		case 65:
			goto st39
		case 95:
			goto tr37
		case 97:
			goto st39
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st39:
		if p++; p == pe {
			goto _test_eof39
		}
	st_case_39:
		switch data[p] {
		case 76:
			goto tr63
		case 95:
			goto tr37
		case 108:
			goto tr63
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st40:
		if p++; p == pe {
			goto _test_eof40
		}
	st_case_40:
		switch data[p] {
		case 69:
			goto st41
		case 95:
			goto tr37
		case 101:
			goto st41
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st41:
		if p++; p == pe {
			goto _test_eof41
		}
	st_case_41:
		switch data[p] {
		case 69:
			goto st42
		case 95:
			goto tr37
		case 101:
			goto st42
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st42:
		if p++; p == pe {
			goto _test_eof42
		}
	st_case_42:
		switch data[p] {
		case 82:
			goto st43
		case 95:
			goto tr37
		case 114:
			goto st43
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st43:
		if p++; p == pe {
			goto _test_eof43
		}
	st_case_43:
		switch data[p] {
		case 83:
			goto st44
		case 95:
			goto tr37
		case 115:
			goto st44
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st44:
		if p++; p == pe {
			goto _test_eof44
		}
	st_case_44:
		if data[p] == 95 {
			goto st45
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr68
	st45:
		if p++; p == pe {
			goto _test_eof45
		}
	st_case_45:
		switch data[p] {
		case 86:
			goto st46
		case 95:
			goto tr37
		case 118:
			goto st46
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st46:
		if p++; p == pe {
			goto _test_eof46
		}
	st_case_46:
		switch data[p] {
		case 50:
			goto tr71
		case 95:
			goto tr37
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st47:
		if p++; p == pe {
			goto _test_eof47
		}
	st_case_47:
		switch data[p] {
		case 69:
			goto st48
		case 89:
			goto st52
		case 95:
			goto tr37
		case 101:
			goto st48
		case 121:
			goto st52
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st48:
		if p++; p == pe {
			goto _test_eof48
		}
	st_case_48:
		switch data[p] {
		case 76:
			goto st49
		case 95:
			goto tr37
		case 108:
			goto st49
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st49:
		if p++; p == pe {
			goto _test_eof49
		}
	st_case_49:
		switch data[p] {
		case 69:
			goto st50
		case 95:
			goto tr37
		case 101:
			goto st50
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st50:
		if p++; p == pe {
			goto _test_eof50
		}
	st_case_50:
		switch data[p] {
		case 67:
			goto st51
		case 95:
			goto tr37
		case 99:
			goto st51
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st51:
		if p++; p == pe {
			goto _test_eof51
		}
	st_case_51:
		switch data[p] {
		case 84:
			goto tr77
		case 95:
			goto tr37
		case 116:
			goto tr77
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st52:
		if p++; p == pe {
			goto _test_eof52
		}
	st_case_52:
		switch data[p] {
		case 83:
			goto st53
		case 95:
			goto tr37
		case 115:
			goto st53
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st53:
		if p++; p == pe {
			goto _test_eof53
		}
	st_case_53:
		switch data[p] {
		case 84:
			goto st54
		case 95:
			goto tr37
		case 116:
			goto st54
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st54:
		if p++; p == pe {
			goto _test_eof54
		}
	st_case_54:
		switch data[p] {
		case 69:
			goto st55
		case 95:
			goto tr37
		case 101:
			goto st55
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st55:
		if p++; p == pe {
			goto _test_eof55
		}
	st_case_55:
		switch data[p] {
		case 77:
			goto tr81
		case 95:
			goto tr37
		case 109:
			goto tr81
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st56:
		if p++; p == pe {
			goto _test_eof56
		}
	st_case_56:
		switch data[p] {
		case 83:
			goto st57
		case 95:
			goto tr37
		case 115:
			goto st57
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st57:
		if p++; p == pe {
			goto _test_eof57
		}
	st_case_57:
		switch data[p] {
		case 69:
			goto tr83
		case 95:
			goto tr37
		case 101:
			goto tr83
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr37
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr37
			}
		default:
			goto tr37
		}
		goto tr48
	st_out:
	_test_eof21: cs = 21; goto _test_eof
	_test_eof22: cs = 22; goto _test_eof
	_test_eof23: cs = 23; goto _test_eof
	_test_eof0: cs = 0; goto _test_eof
	_test_eof1: cs = 1; goto _test_eof
	_test_eof24: cs = 24; goto _test_eof
	_test_eof2: cs = 2; goto _test_eof
	_test_eof3: cs = 3; goto _test_eof
	_test_eof4: cs = 4; goto _test_eof
	_test_eof5: cs = 5; goto _test_eof
	_test_eof6: cs = 6; goto _test_eof
	_test_eof7: cs = 7; goto _test_eof
	_test_eof8: cs = 8; goto _test_eof
	_test_eof9: cs = 9; goto _test_eof
	_test_eof10: cs = 10; goto _test_eof
	_test_eof11: cs = 11; goto _test_eof
	_test_eof12: cs = 12; goto _test_eof
	_test_eof13: cs = 13; goto _test_eof
	_test_eof14: cs = 14; goto _test_eof
	_test_eof15: cs = 15; goto _test_eof
	_test_eof16: cs = 16; goto _test_eof
	_test_eof17: cs = 17; goto _test_eof
	_test_eof18: cs = 18; goto _test_eof
	_test_eof19: cs = 19; goto _test_eof
	_test_eof20: cs = 20; goto _test_eof
	_test_eof25: cs = 25; goto _test_eof
	_test_eof26: cs = 26; goto _test_eof
	_test_eof27: cs = 27; goto _test_eof
	_test_eof28: cs = 28; goto _test_eof
	_test_eof29: cs = 29; goto _test_eof
	_test_eof30: cs = 30; goto _test_eof
	_test_eof31: cs = 31; goto _test_eof
	_test_eof32: cs = 32; goto _test_eof
	_test_eof33: cs = 33; goto _test_eof
	_test_eof34: cs = 34; goto _test_eof
	_test_eof35: cs = 35; goto _test_eof
	_test_eof36: cs = 36; goto _test_eof
	_test_eof37: cs = 37; goto _test_eof
	_test_eof38: cs = 38; goto _test_eof
	_test_eof39: cs = 39; goto _test_eof
	_test_eof40: cs = 40; goto _test_eof
	_test_eof41: cs = 41; goto _test_eof
	_test_eof42: cs = 42; goto _test_eof
	_test_eof43: cs = 43; goto _test_eof
	_test_eof44: cs = 44; goto _test_eof
	_test_eof45: cs = 45; goto _test_eof
	_test_eof46: cs = 46; goto _test_eof
	_test_eof47: cs = 47; goto _test_eof
	_test_eof48: cs = 48; goto _test_eof
	_test_eof49: cs = 49; goto _test_eof
	_test_eof50: cs = 50; goto _test_eof
	_test_eof51: cs = 51; goto _test_eof
	_test_eof52: cs = 52; goto _test_eof
	_test_eof53: cs = 53; goto _test_eof
	_test_eof54: cs = 54; goto _test_eof
	_test_eof55: cs = 55; goto _test_eof
	_test_eof56: cs = 56; goto _test_eof
	_test_eof57: cs = 57; goto _test_eof

	_test_eof: {}
	if p == eof {
		switch cs {
		case 22:
			goto tr44
		case 23:
			goto tr44
		case 0:
			goto tr0
		case 1:
			goto tr0
		case 24:
			goto tr48
		case 2:
			goto tr5
		case 3:
			goto tr5
		case 4:
			goto tr5
		case 5:
			goto tr5
		case 6:
			goto tr5
		case 7:
			goto tr5
		case 8:
			goto tr5
		case 9:
			goto tr5
		case 10:
			goto tr5
		case 11:
			goto tr5
		case 12:
			goto tr5
		case 13:
			goto tr5
		case 14:
			goto tr5
		case 15:
			goto tr5
		case 16:
			goto tr5
		case 17:
			goto tr5
		case 18:
			goto tr5
		case 19:
			goto tr5
		case 20:
			goto tr5
		case 25:
			goto tr48
		case 26:
			goto tr0
		case 27:
			goto tr48
		case 28:
			goto tr48
		case 29:
			goto tr48
		case 30:
			goto tr48
		case 31:
			goto tr48
		case 32:
			goto tr48
		case 33:
			goto tr48
		case 34:
			goto tr48
		case 35:
			goto tr48
		case 36:
			goto tr48
		case 37:
			goto tr48
		case 38:
			goto tr48
		case 39:
			goto tr48
		case 40:
			goto tr48
		case 41:
			goto tr48
		case 42:
			goto tr48
		case 43:
			goto tr48
		case 44:
			goto tr68
		case 45:
			goto tr48
		case 46:
			goto tr48
		case 47:
			goto tr48
		case 48:
			goto tr48
		case 49:
			goto tr48
		case 50:
			goto tr48
		case 51:
			goto tr48
		case 52:
			goto tr48
		case 53:
			goto tr48
		case 54:
			goto tr48
		case 55:
			goto tr48
		case 56:
			goto tr48
		case 57:
			goto tr48
		}
	}

	_out: {}
	}

//line lexer.rl:94


    l.p = p

    return token
}