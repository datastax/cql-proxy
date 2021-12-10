
//line lexer.rl:1
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


//line lexer.go:36
const lex_start int = 2
const lex_first_final int = 2
const lex_error int = -1

const lex_en_main int = 2


//line lexer.rl:35


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

    
//line lexer.go:82
	{
	cs = lex_start
	ts = 0
	te = 0
	act = 0
	}

//line lexer.go:90
	{
	if p == pe {
		goto _test_eof
	}
	switch cs {
	case 2:
		goto st_case_2
	case 3:
		goto st_case_3
	case 4:
		goto st_case_4
	case 0:
		goto st_case_0
	case 1:
		goto st_case_1
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
	case 21:
		goto st_case_21
	case 22:
		goto st_case_22
	case 23:
		goto st_case_23
	case 24:
		goto st_case_24
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
	}
	goto st_out
tr0:
//line NONE:1
	switch act {
	case 1:
	{p = (te) - 1
 tk = tkSelect; {p++; cs = 2; goto _out } }
	case 2:
	{p = (te) - 1
 tk = tkInsert; {p++; cs = 2; goto _out } }
	case 3:
	{p = (te) - 1
 tk = tkUpdate; {p++; cs = 2; goto _out } }
	case 4:
	{p = (te) - 1
 tk = tkDelete; {p++; cs = 2; goto _out } }
	case 5:
	{p = (te) - 1
 tk = tkBatch; {p++; cs = 2; goto _out } }
	case 6:
	{p = (te) - 1
 tk = tkBegin; {p++; cs = 2; goto _out } }
	case 7:
	{p = (te) - 1
 tk = tkApply; {p++; cs = 2; goto _out } }
	case 8:
	{p = (te) - 1
 tk = tkInto; {p++; cs = 2; goto _out } }
	case 9:
	{p = (te) - 1
 tk = tkValues; {p++; cs = 2; goto _out } }
	case 10:
	{p = (te) - 1
 tk = tkSet; {p++; cs = 2; goto _out } }
	case 11:
	{p = (te) - 1
 tk = tkFrom; {p++; cs = 2; goto _out } }
	case 12:
	{p = (te) - 1
 tk = tkUse; {p++; cs = 2; goto _out } }
	case 13:
	{p = (te) - 1
 tk = tkIf; {p++; cs = 2; goto _out } }
	case 14:
	{p = (te) - 1
 tk = tkAs; {p++; cs = 2; goto _out } }
	case 15:
	{p = (te) - 1
 tk = tkCount; {p++; cs = 2; goto _out } }
	case 16:
	{p = (te) - 1
 tk = tkCast; {p++; cs = 2; goto _out } }
	case 22:
	{p = (te) - 1
 tk = tkIdentifier; l.c = l.data[ts:te]; {p++; cs = 2; goto _out } }
	case 25:
	{p = (te) - 1
 tk = tkInvalid; {p++; cs = 2; goto _out } }
	}
	
	goto st2
tr2:
//line lexer.rl:98
te = p+1
{ tk = tkIdentifier; l.c = l.data[ts:te]; {p++; cs = 2; goto _out } }
	goto st2
tr5:
//line lexer.rl:101
te = p+1
{ tk = tkInvalid; {p++; cs = 2; goto _out } }
	goto st2
tr6:
//line lexer.rl:100
te = p+1
{ /* Skip */ }
	goto st2
tr7:
//line lexer.rl:99
te = p+1
{ /* Skip */ }
	goto st2
tr10:
//line lexer.rl:96
te = p+1
{ tk = tkLparen; {p++; cs = 2; goto _out } }
	goto st2
tr11:
//line lexer.rl:97
te = p+1
{ tk = tkRparen; {p++; cs = 2; goto _out } }
	goto st2
tr12:
//line lexer.rl:93
te = p+1
{ tk = tkStar; {p++; cs = 2; goto _out } }
	goto st2
tr13:
//line lexer.rl:94
te = p+1
{ tk = tkComma; {p++; cs = 2; goto _out } }
	goto st2
tr14:
//line lexer.rl:95
te = p+1
{ tk = tkDot; {p++; cs = 2; goto _out } }
	goto st2
tr25:
//line lexer.rl:101
te = p
p--
{ tk = tkInvalid; {p++; cs = 2; goto _out } }
	goto st2
tr26:
//line lexer.rl:98
te = p
p--
{ tk = tkIdentifier; l.c = l.data[ts:te]; {p++; cs = 2; goto _out } }
	goto st2
	st2:
//line NONE:1
ts = 0

		if p++; p == pe {
			goto _test_eof2
		}
	st_case_2:
//line NONE:1
ts = p

//line lexer.go:332
		switch data[p] {
		case 9:
			goto tr6
		case 10:
			goto tr7
		case 13:
			goto st3
		case 32:
			goto tr6
		case 34:
			goto tr9
		case 40:
			goto tr10
		case 41:
			goto tr11
		case 42:
			goto tr12
		case 44:
			goto tr13
		case 46:
			goto tr14
		case 65:
			goto st5
		case 66:
			goto st10
		case 67:
			goto st17
		case 68:
			goto st23
		case 70:
			goto st28
		case 73:
			goto st31
		case 83:
			goto st37
		case 85:
			goto st42
		case 86:
			goto st48
		case 97:
			goto st5
		case 98:
			goto st10
		case 99:
			goto st17
		case 100:
			goto st23
		case 102:
			goto st28
		case 105:
			goto st31
		case 115:
			goto st37
		case 117:
			goto st42
		case 118:
			goto st48
		}
		switch {
		case data[p] > 90:
			if 101 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		case data[p] >= 69:
			goto tr19
		}
		goto tr5
	st3:
		if p++; p == pe {
			goto _test_eof3
		}
	st_case_3:
		if data[p] == 10 {
			goto tr7
		}
		goto tr25
tr4:
//line NONE:1
te = p+1

//line lexer.rl:98
act = 22;
	goto st4
tr9:
//line NONE:1
te = p+1

//line lexer.rl:101
act = 25;
	goto st4
	st4:
		if p++; p == pe {
			goto _test_eof4
		}
	st_case_4:
//line lexer.go:428
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
	st5:
		if p++; p == pe {
			goto _test_eof5
		}
	st_case_5:
		switch data[p] {
		case 80:
			goto st7
		case 83:
			goto tr28
		case 95:
			goto tr19
		case 112:
			goto st7
		case 115:
			goto tr28
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
tr19:
//line NONE:1
te = p+1

//line lexer.rl:98
act = 22;
	goto st6
tr28:
//line NONE:1
te = p+1

//line lexer.rl:90
act = 14;
	goto st6
tr31:
//line NONE:1
te = p+1

//line lexer.rl:83
act = 7;
	goto st6
tr36:
//line NONE:1
te = p+1

//line lexer.rl:81
act = 5;
	goto st6
tr39:
//line NONE:1
te = p+1

//line lexer.rl:82
act = 6;
	goto st6
tr43:
//line NONE:1
te = p+1

//line lexer.rl:92
act = 16;
	goto st6
tr46:
//line NONE:1
te = p+1

//line lexer.rl:91
act = 15;
	goto st6
tr51:
//line NONE:1
te = p+1

//line lexer.rl:80
act = 4;
	goto st6
tr54:
//line NONE:1
te = p+1

//line lexer.rl:87
act = 11;
	goto st6
tr55:
//line NONE:1
te = p+1

//line lexer.rl:89
act = 13;
	goto st6
tr61:
//line NONE:1
te = p+1

//line lexer.rl:78
act = 2;
	goto st6
tr62:
//line NONE:1
te = p+1

//line lexer.rl:84
act = 8;
	goto st6
tr65:
//line NONE:1
te = p+1

//line lexer.rl:86
act = 10;
	goto st6
tr68:
//line NONE:1
te = p+1

//line lexer.rl:77
act = 1;
	goto st6
tr74:
//line NONE:1
te = p+1

//line lexer.rl:79
act = 3;
	goto st6
tr75:
//line NONE:1
te = p+1

//line lexer.rl:88
act = 12;
	goto st6
tr80:
//line NONE:1
te = p+1

//line lexer.rl:85
act = 9;
	goto st6
	st6:
		if p++; p == pe {
			goto _test_eof6
		}
	st_case_6:
//line lexer.go:626
		if data[p] == 95 {
			goto tr19
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr0
	st7:
		if p++; p == pe {
			goto _test_eof7
		}
	st_case_7:
		switch data[p] {
		case 80:
			goto st8
		case 95:
			goto tr19
		case 112:
			goto st8
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st8:
		if p++; p == pe {
			goto _test_eof8
		}
	st_case_8:
		switch data[p] {
		case 76:
			goto st9
		case 95:
			goto tr19
		case 108:
			goto st9
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st9:
		if p++; p == pe {
			goto _test_eof9
		}
	st_case_9:
		switch data[p] {
		case 89:
			goto tr31
		case 95:
			goto tr19
		case 121:
			goto tr31
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st10:
		if p++; p == pe {
			goto _test_eof10
		}
	st_case_10:
		switch data[p] {
		case 65:
			goto st11
		case 69:
			goto st14
		case 95:
			goto tr19
		case 97:
			goto st11
		case 101:
			goto st14
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st11:
		if p++; p == pe {
			goto _test_eof11
		}
	st_case_11:
		switch data[p] {
		case 84:
			goto st12
		case 95:
			goto tr19
		case 116:
			goto st12
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st12:
		if p++; p == pe {
			goto _test_eof12
		}
	st_case_12:
		switch data[p] {
		case 67:
			goto st13
		case 95:
			goto tr19
		case 99:
			goto st13
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st13:
		if p++; p == pe {
			goto _test_eof13
		}
	st_case_13:
		switch data[p] {
		case 72:
			goto tr36
		case 95:
			goto tr19
		case 104:
			goto tr36
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st14:
		if p++; p == pe {
			goto _test_eof14
		}
	st_case_14:
		switch data[p] {
		case 71:
			goto st15
		case 95:
			goto tr19
		case 103:
			goto st15
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st15:
		if p++; p == pe {
			goto _test_eof15
		}
	st_case_15:
		switch data[p] {
		case 73:
			goto st16
		case 95:
			goto tr19
		case 105:
			goto st16
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st16:
		if p++; p == pe {
			goto _test_eof16
		}
	st_case_16:
		switch data[p] {
		case 78:
			goto tr39
		case 95:
			goto tr19
		case 110:
			goto tr39
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st17:
		if p++; p == pe {
			goto _test_eof17
		}
	st_case_17:
		switch data[p] {
		case 65:
			goto st18
		case 79:
			goto st20
		case 95:
			goto tr19
		case 97:
			goto st18
		case 111:
			goto st20
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st18:
		if p++; p == pe {
			goto _test_eof18
		}
	st_case_18:
		switch data[p] {
		case 83:
			goto st19
		case 95:
			goto tr19
		case 115:
			goto st19
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st19:
		if p++; p == pe {
			goto _test_eof19
		}
	st_case_19:
		switch data[p] {
		case 84:
			goto tr43
		case 95:
			goto tr19
		case 116:
			goto tr43
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st20:
		if p++; p == pe {
			goto _test_eof20
		}
	st_case_20:
		switch data[p] {
		case 85:
			goto st21
		case 95:
			goto tr19
		case 117:
			goto st21
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st21:
		if p++; p == pe {
			goto _test_eof21
		}
	st_case_21:
		switch data[p] {
		case 78:
			goto st22
		case 95:
			goto tr19
		case 110:
			goto st22
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st22:
		if p++; p == pe {
			goto _test_eof22
		}
	st_case_22:
		switch data[p] {
		case 84:
			goto tr46
		case 95:
			goto tr19
		case 116:
			goto tr46
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st23:
		if p++; p == pe {
			goto _test_eof23
		}
	st_case_23:
		switch data[p] {
		case 69:
			goto st24
		case 95:
			goto tr19
		case 101:
			goto st24
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st24:
		if p++; p == pe {
			goto _test_eof24
		}
	st_case_24:
		switch data[p] {
		case 76:
			goto st25
		case 95:
			goto tr19
		case 108:
			goto st25
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st25:
		if p++; p == pe {
			goto _test_eof25
		}
	st_case_25:
		switch data[p] {
		case 69:
			goto st26
		case 95:
			goto tr19
		case 101:
			goto st26
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st26:
		if p++; p == pe {
			goto _test_eof26
		}
	st_case_26:
		switch data[p] {
		case 84:
			goto st27
		case 95:
			goto tr19
		case 116:
			goto st27
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st27:
		if p++; p == pe {
			goto _test_eof27
		}
	st_case_27:
		switch data[p] {
		case 69:
			goto tr51
		case 95:
			goto tr19
		case 101:
			goto tr51
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st28:
		if p++; p == pe {
			goto _test_eof28
		}
	st_case_28:
		switch data[p] {
		case 82:
			goto st29
		case 95:
			goto tr19
		case 114:
			goto st29
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st29:
		if p++; p == pe {
			goto _test_eof29
		}
	st_case_29:
		switch data[p] {
		case 79:
			goto st30
		case 95:
			goto tr19
		case 111:
			goto st30
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st30:
		if p++; p == pe {
			goto _test_eof30
		}
	st_case_30:
		switch data[p] {
		case 77:
			goto tr54
		case 95:
			goto tr19
		case 109:
			goto tr54
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st31:
		if p++; p == pe {
			goto _test_eof31
		}
	st_case_31:
		switch data[p] {
		case 70:
			goto tr55
		case 78:
			goto st32
		case 95:
			goto tr19
		case 102:
			goto tr55
		case 110:
			goto st32
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st32:
		if p++; p == pe {
			goto _test_eof32
		}
	st_case_32:
		switch data[p] {
		case 83:
			goto st33
		case 84:
			goto st36
		case 95:
			goto tr19
		case 115:
			goto st33
		case 116:
			goto st36
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st33:
		if p++; p == pe {
			goto _test_eof33
		}
	st_case_33:
		switch data[p] {
		case 69:
			goto st34
		case 95:
			goto tr19
		case 101:
			goto st34
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st34:
		if p++; p == pe {
			goto _test_eof34
		}
	st_case_34:
		switch data[p] {
		case 82:
			goto st35
		case 95:
			goto tr19
		case 114:
			goto st35
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st35:
		if p++; p == pe {
			goto _test_eof35
		}
	st_case_35:
		switch data[p] {
		case 84:
			goto tr61
		case 95:
			goto tr19
		case 116:
			goto tr61
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st36:
		if p++; p == pe {
			goto _test_eof36
		}
	st_case_36:
		switch data[p] {
		case 79:
			goto tr62
		case 95:
			goto tr19
		case 111:
			goto tr62
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st37:
		if p++; p == pe {
			goto _test_eof37
		}
	st_case_37:
		switch data[p] {
		case 69:
			goto st38
		case 95:
			goto tr19
		case 101:
			goto st38
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st38:
		if p++; p == pe {
			goto _test_eof38
		}
	st_case_38:
		switch data[p] {
		case 76:
			goto st39
		case 84:
			goto tr65
		case 95:
			goto tr19
		case 108:
			goto st39
		case 116:
			goto tr65
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st39:
		if p++; p == pe {
			goto _test_eof39
		}
	st_case_39:
		switch data[p] {
		case 69:
			goto st40
		case 95:
			goto tr19
		case 101:
			goto st40
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st40:
		if p++; p == pe {
			goto _test_eof40
		}
	st_case_40:
		switch data[p] {
		case 67:
			goto st41
		case 95:
			goto tr19
		case 99:
			goto st41
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st41:
		if p++; p == pe {
			goto _test_eof41
		}
	st_case_41:
		switch data[p] {
		case 84:
			goto tr68
		case 95:
			goto tr19
		case 116:
			goto tr68
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st42:
		if p++; p == pe {
			goto _test_eof42
		}
	st_case_42:
		switch data[p] {
		case 80:
			goto st43
		case 83:
			goto st47
		case 95:
			goto tr19
		case 112:
			goto st43
		case 115:
			goto st47
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st43:
		if p++; p == pe {
			goto _test_eof43
		}
	st_case_43:
		switch data[p] {
		case 68:
			goto st44
		case 95:
			goto tr19
		case 100:
			goto st44
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st44:
		if p++; p == pe {
			goto _test_eof44
		}
	st_case_44:
		switch data[p] {
		case 65:
			goto st45
		case 95:
			goto tr19
		case 97:
			goto st45
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st45:
		if p++; p == pe {
			goto _test_eof45
		}
	st_case_45:
		switch data[p] {
		case 84:
			goto st46
		case 95:
			goto tr19
		case 116:
			goto st46
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st46:
		if p++; p == pe {
			goto _test_eof46
		}
	st_case_46:
		switch data[p] {
		case 69:
			goto tr74
		case 95:
			goto tr19
		case 101:
			goto tr74
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st47:
		if p++; p == pe {
			goto _test_eof47
		}
	st_case_47:
		switch data[p] {
		case 69:
			goto tr75
		case 95:
			goto tr19
		case 101:
			goto tr75
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st48:
		if p++; p == pe {
			goto _test_eof48
		}
	st_case_48:
		switch data[p] {
		case 65:
			goto st49
		case 95:
			goto tr19
		case 97:
			goto st49
		}
		switch {
		case data[p] < 66:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 98 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st49:
		if p++; p == pe {
			goto _test_eof49
		}
	st_case_49:
		switch data[p] {
		case 76:
			goto st50
		case 95:
			goto tr19
		case 108:
			goto st50
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st50:
		if p++; p == pe {
			goto _test_eof50
		}
	st_case_50:
		switch data[p] {
		case 85:
			goto st51
		case 95:
			goto tr19
		case 117:
			goto st51
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st51:
		if p++; p == pe {
			goto _test_eof51
		}
	st_case_51:
		switch data[p] {
		case 69:
			goto st52
		case 95:
			goto tr19
		case 101:
			goto st52
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st52:
		if p++; p == pe {
			goto _test_eof52
		}
	st_case_52:
		switch data[p] {
		case 83:
			goto tr80
		case 95:
			goto tr19
		case 115:
			goto tr80
		}
		switch {
		case data[p] < 65:
			if 48 <= data[p] && data[p] <= 57 {
				goto tr19
			}
		case data[p] > 90:
			if 97 <= data[p] && data[p] <= 122 {
				goto tr19
			}
		default:
			goto tr19
		}
		goto tr26
	st_out:
	_test_eof2: cs = 2; goto _test_eof
	_test_eof3: cs = 3; goto _test_eof
	_test_eof4: cs = 4; goto _test_eof
	_test_eof0: cs = 0; goto _test_eof
	_test_eof1: cs = 1; goto _test_eof
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
	_test_eof21: cs = 21; goto _test_eof
	_test_eof22: cs = 22; goto _test_eof
	_test_eof23: cs = 23; goto _test_eof
	_test_eof24: cs = 24; goto _test_eof
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

	_test_eof: {}
	if p == eof {
		switch cs {
		case 3:
			goto tr25
		case 4:
			goto tr0
		case 0:
			goto tr0
		case 1:
			goto tr0
		case 5:
			goto tr26
		case 6:
			goto tr0
		case 7:
			goto tr26
		case 8:
			goto tr26
		case 9:
			goto tr26
		case 10:
			goto tr26
		case 11:
			goto tr26
		case 12:
			goto tr26
		case 13:
			goto tr26
		case 14:
			goto tr26
		case 15:
			goto tr26
		case 16:
			goto tr26
		case 17:
			goto tr26
		case 18:
			goto tr26
		case 19:
			goto tr26
		case 20:
			goto tr26
		case 21:
			goto tr26
		case 22:
			goto tr26
		case 23:
			goto tr26
		case 24:
			goto tr26
		case 25:
			goto tr26
		case 26:
			goto tr26
		case 27:
			goto tr26
		case 28:
			goto tr26
		case 29:
			goto tr26
		case 30:
			goto tr26
		case 31:
			goto tr26
		case 32:
			goto tr26
		case 33:
			goto tr26
		case 34:
			goto tr26
		case 35:
			goto tr26
		case 36:
			goto tr26
		case 37:
			goto tr26
		case 38:
			goto tr26
		case 39:
			goto tr26
		case 40:
			goto tr26
		case 41:
			goto tr26
		case 42:
			goto tr26
		case 43:
			goto tr26
		case 44:
			goto tr26
		case 45:
			goto tr26
		case 46:
			goto tr26
		case 47:
			goto tr26
		case 48:
			goto tr26
		case 49:
			goto tr26
		case 50:
			goto tr26
		case 51:
			goto tr26
		case 52:
			goto tr26
		}
	}

	_out: {}
	}

//line lexer.rl:106


    l.p = p

    return tk
}