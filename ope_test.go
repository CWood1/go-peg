package peg

import "testing"

type Cases []struct {
	input string
	want  int
}

func run(name string, t *testing.T, ope Ope, cases Cases) {
	for _, cs := range cases {
		sv := &SemanticValues{}
		c := &context{}
		if got := ope.parse(cs.input, sv, c, nil); got != cs.want {
			t.Errorf("[%s] input:%q want:%d got:%d", name, cs.input, cs.want, got)
		}
	}
}

func TestSequence(t *testing.T) {
	ope := Seq(
		Lit("日本語"),
		Lit("も"),
		Lit("OK"),
		Lit("です。"),
	)
	cases := Cases{
		{"日本語もOKです。", 23},
		{"日本語OKです。", -1},
	}
	run("Sequence", t, ope, cases)
}

func TestPrioritizedChoice(t *testing.T) {
	ope := Cho(
		Lit("English"),
		Lit("日本語"),
	)
	cases := Cases{
		{"日本語", 9},
		{"English", 7},
		{"Go", -1},
	}
	run("PrioritizedChoice", t, ope, cases)
}

func TestZeroOrMore(t *testing.T) {
	ope := Zom(
		Lit("abc"),
	)
	cases := Cases{
		{"", 0},
		{"a", 0},
		{"b", 0},
		{"ab", 0},
		{"abc", 3},
		{"abca", 3},
		{"abcabc", 6},
	}
	run("ZeroOrMore", t, ope, cases)
}

func TestOneOrMore(t *testing.T) {
	ope := Oom(
		Lit("abc"),
	)
	cases := Cases{
		{"", -1},
		{"a", -1},
		{"b", -1},
		{"ab", -1},
		{"abc", 3},
		{"abca", 3},
		{"abcabc", 6},
	}
	run("OneOrMore", t, ope, cases)
}

func TestOption(t *testing.T) {
	ope := Opt(
		Lit("abc"),
	)
	cases := Cases{
		{"", 0},
		{"a", 0},
		{"b", 0},
		{"ab", 0},
		{"abc", 3},
		{"abca", 3},
		{"abcabc", 3},
	}
	run("Option", t, ope, cases)
}

func TestAndPredicate(t *testing.T) {
	ope := Apd(
		Lit("abc"),
	)
	cases := Cases{
		{"", -1},
		{"a", -1},
		{"b", -1},
		{"ab", -1},
		{"abc", 0},
		{"abca", 0},
		{"abcabc", 0},
	}
	run("AndPredicate", t, ope, cases)
}

func TestNotPredicate(t *testing.T) {
	ope := Npd(
		Lit("abc"),
	)
	cases := Cases{
		{"", 0},
		{"a", 0},
		{"b", 0},
		{"ab", 0},
		{"abc", -1},
		{"abca", -1},
		{"abcabc", -1},
	}
	run("NotPredicate", t, ope, cases)
}

func TestLiteralString(t *testing.T) {
	ope := Lit("日本語")
	cases := Cases{
		{"", -1},
		{"日", -1},
		{"日本語", 9},
		{"日本語です。", 9},
		{"English", -1},
	}
	run("LiteralString", t, ope, cases)
}

func TestCharacterClass(t *testing.T) {
	ope := Cls("a-zA-Z0-9_")
	cases := Cases{
		{"", -1},
		{"a", 1},
		{"b", 1},
		{"z", 1},
		{"A", 1},
		{"B", 1},
		{"Z", 1},
		{"0", 1},
		{"1", 1},
		{"9", 1},
		{"_", 1},
		{"-", -1},
		{" ", -1},
	}
	run("CharacterClass", t, ope, cases)
}

func TestTokenBoundary(t *testing.T) {
	ope := Seq(Tok(Lit("hello")), Lit(" "))
	sv := &SemanticValues{}
	c := &context{}
	input := "hello "

	want := len(input)
	if got := ope.parse(input, sv, c, nil); got != want {
		t.Errorf("[%s] input:%q want:%d got:%d", "TokenBoundary", input, want, got)
	}

	tok := "hello"
	if sv.isValidString == false || sv.S != tok {
		t.Errorf("[%s] input:%q want:%d got:%d", "TokenBoundary", input, tok, sv.S)
	}
}

func TestIgnore(t *testing.T) {
	var NUMBER, WS Rule
	NUMBER.Ope = Seq(Tok(Oom(Cls("0-9"))), Ign(&WS))
	WS.Ope = Zom(Cls(" \t"))

	input := "123 "

	NUMBER.Action = func(sv *SemanticValues, dt Any) (v Any, err error) {
		n := 0
		if len(sv.Vs) != n {
			t.Errorf("[%s] input:%q want:%d got:%d", "Ignore", input, n, len(sv.Vs))
		}
		return
	}

	want := len(input)
	if l, _, _ := NUMBER.Parse(input, nil); l != want {
		t.Errorf("[%s] input:%q want:%d got:%d", "Ignore", input, want, l)
	}
}
