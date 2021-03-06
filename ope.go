package peg

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Sequence Error
type SequenceError struct {
	Errs []error
}

func (s SequenceError) Error() string {
	str := "Encountered a sequence error. Choices are:\n===\n\n"

	var errs []string

	for _, err := range s.Errs {
		errs = append(errs, err.Error())
	}

	str = str + strings.Join(errs, "\n\n") + "\n===\n\n"
	return str
}

// Operator Error
type OperatorError struct {
	Expected []string
	Got      string
	Line     int
	Col      int
	Length   int
}

func (o OperatorError) Error() string {
	lineStart, lineEnd := printLine(o.Got, o.Line)
	str := fmt.Sprintf("%d:%d  ", o.Line, o.Col)
	str = str + o.Got[lineStart:lineEnd] + "\n" +
		strings.Repeat("-", len(str)+o.Col-1)
	str = str + strings.Repeat("^", o.Length) + "\n\n"
	str = str + "Expected "

	if len(o.Expected) > 1 {
		str = str + "one of \"" + strings.Join(o.Expected, "\", \"")
	} else {
		str = str + "\"" + o.Expected[0]
	}

	endOfToken := lineStart + (o.Col - 1) + o.Length

	if endOfToken > len(o.Got) {
		endOfToken = len(o.Got)
	}

	str = str + "\", instead got \"" + o.Got[lineStart+(o.Col-1):endOfToken] +
		"\".\n"
	return "\n" + str
}

// Any
type Any interface {
}

// Token
type Token struct {
	Pos int
	S   string
}

// Semantic values
type Values struct {
	SS     string
	Vs     []Any
	Pos    int
	S      string
	Choice int
	Ts     []Token
}

func (v *Values) Len() int {
	return len(v.Vs)
}

func (v *Values) ToStr(i int) string {
	return v.Vs[i].(string)
}

func (v *Values) ToInt(i int) int {
	return v.Vs[i].(int)
}

func (v *Values) ToFloat32(i int) float32 {
	return v.Vs[i].(float32)
}

func (v *Values) ToFloat64(i int) float64 {
	return v.Vs[i].(float64)
}

func (v *Values) ToBool(i int) bool {
	return v.Vs[i].(bool)
}

func (v *Values) ToOpe(i int) operator {
	return v.Vs[i].(operator)
}

func (v *Values) Token() string {
	if len(v.Ts) > 0 {
		return v.Ts[0].S
	}
	return v.S
}

// Context
type context struct {
	s string

	errorPos   int
	messagePos int
	message    string

	svStack   []Values
	argsStack [][]operator

	inToken bool

	whitespaceOpe operator
	inWhitespace  bool

	wordOpe operator

	tracerEnter func(name string, s string, v *Values, d Any, p int)
	tracerLeave func(name string, s string, v *Values, d Any, p int, l int)
}

func (c *context) setErrorPos(p int) {
	if c.errorPos < p {
		c.errorPos = p
	}
}

func (c *context) push() *Values {
	v := Values{SS: c.s}
	c.svStack = append(c.svStack, v)
	return &c.svStack[len(c.svStack)-1]
}

func (c *context) pop() {
	c.svStack = c.svStack[:len(c.svStack)-1]
}

func (c *context) pushArgs(args []operator) {
	c.argsStack = append(c.argsStack, args)
}

func (c *context) popArgs() {
	c.argsStack = c.argsStack[:len(c.argsStack)-1]
}

func (c *context) topArg() []operator {
	if len(c.argsStack) == 0 {
		return nil
	}
	return c.argsStack[len(c.argsStack)-1]
}

// parse
func parse(o operator, s string, p int, v *Values, c *context, d Any) (l int, err error) {
	if c.tracerEnter != nil {
		c.tracerEnter(o.Label(), s, v, d, p)
	}

	l, err = o.parseCore(s, p, v, c, d)

	if c.tracerLeave != nil {
		c.tracerLeave(o.Label(), s, v, d, p, l)
	}
	return
}

// Operator
type operator interface {
	Label() string
	parse(s string, p int, v *Values, c *context, d Any) (int, error)
	parseCore(s string, p int, v *Values, c *context, d Any) (int, error)
	accept(v visitor)
}

// Operator base
type opeBase struct {
	derived operator
}

func (o *opeBase) Label() string {
	return reflect.TypeOf(o.derived).String()[5:]
}

func (o *opeBase) parse(s string, p int, v *Values, c *context, d Any) (int, error) {
	return parse(o.derived, s, p, v, c, d)
}

// Sequence
type sequence struct {
	opeBase
	opes []operator
}

func (o *sequence) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	l = 0
	var chl int
	for _, ope := range o.opes {
		chl, err = ope.parse(s, p+l, v, c, d)
		if err != nil {
			l = 0
			return
		}
		l += chl
	}

	return
}

func (o *sequence) accept(v visitor) {
	v.visitSequence(o)
}

// Prioritized Choice
type prioritizedChoice struct {
	opeBase
	opes []operator
}

func (o *prioritizedChoice) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	var opeLabels []string
	var errs []error
	var e error
	id := 0
	for _, ope := range o.opes {
		opeLabels = append(opeLabels, ope.Label())
		chv := c.push()
		l, e = ope.parse(s, p, chv, c, d)
		c.pop()
		if e == nil {
			v.Vs = append(v.Vs, chv.Vs...)
			v.Pos = chv.Pos
			v.S = chv.S
			v.Choice = id
			v.Ts = append(v.Ts, chv.Ts...)
			return
		}

		errs = append(errs, e)
		id++
	}

	l = 0
	err = SequenceError{
		errs,
	}

	return
}

func (o *prioritizedChoice) accept(v visitor) {
	v.visitPrioritizedChoice(o)
}

// Zero or More
type zeroOrMore struct {
	opeBase
	ope operator
}

func (o *zeroOrMore) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	saveErrorPos := c.errorPos
	l = 0
	for p+l < len(s) {
		saveVs := v.Vs
		saveTs := v.Ts
		chl, e := o.ope.parse(s, p+l, v, c, d)
		if e != nil {
			v.Vs = saveVs
			v.Ts = saveTs
			c.errorPos = saveErrorPos
			break
		}
		l += chl
	}
	return
}

func (o *zeroOrMore) accept(v visitor) {
	v.visitZeroOrMore(o)
}

// One or More
type oneOrMore struct {
	opeBase
	ope operator
}

func (o *oneOrMore) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	l, err = o.ope.parse(s, p, v, c, d)
	if err != nil {
		return
	}
	saveErrorPos := c.errorPos
	for p+l < len(s) {
		saveVs := v.Vs
		saveTs := v.Ts
		chl, e := o.ope.parse(s, p+l, v, c, d)
		if e != nil {
			v.Vs = saveVs
			v.Ts = saveTs
			c.errorPos = saveErrorPos
			break
		}
		l += chl
	}
	return
}

func (o *oneOrMore) accept(v visitor) {
	v.visitOneOrMore(o)
}

// Option
type option struct {
	opeBase
	ope operator
}

func (o *option) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	saveErrorPos := c.errorPos
	saveVs := v.Vs
	saveTs := v.Ts
	l, err := o.ope.parse(s, p, v, c, d)
	if err != nil {
		v.Vs = saveVs
		v.Ts = saveTs
		c.errorPos = saveErrorPos
		l = 0
		err = nil
	}

	return l, nil
}

func (o *option) accept(v visitor) {
	v.visitOption(o)
}

// And Predicate
type andPredicate struct {
	opeBase
	ope operator
}

func (o *andPredicate) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	chv := c.push()
	_, err = o.ope.parse(s, p, chv, c, d)
	c.pop()

	l = 0
	return
}

func (o *andPredicate) accept(v visitor) {
	v.visitAndPredicate(o)
}

// Not Predicate
type notPredicate struct {
	opeBase
	ope operator
}

func (o *notPredicate) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	saveErrorPos := c.errorPos

	chv := c.push()
	chl, e := o.ope.parse(s, p, chv, c, d)
	c.pop()

	if e == nil {
		c.setErrorPos(p)
		l = 0
		line, col := lineInfo(s, p)
		err = OperatorError{
			[]string{
				"Not " + o.ope.Label(),
			},
			s,
			line,
			col,
			chl,
		}
	} else {
		c.errorPos = saveErrorPos
		l = 0
		err = nil
	}
	return
}

func (o *notPredicate) accept(v visitor) {
	v.visitNotPredicate(o)
}

// Literal String
type literalString struct {
	opeBase
	lit        string
	initIsWord sync.Once
	isWord     bool
}

func (o *literalString) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	l := 0
	for ; l < len(o.lit); l++ {
		if p+l == len(s) || s[p+l] != o.lit[l] {
			c.setErrorPos(p)
			line, col := lineInfo(s, p)

			return 0, OperatorError{
				[]string{
					o.lit,
				},
				s,
				line,
				col,
				len(o.lit),
			}
		}
	}

	// Word check
	o.initIsWord.Do(func() {
		if c.wordOpe != nil {
			_, err := c.wordOpe.parse(o.lit, 0, &Values{}, &context{s: s}, nil)
			o.isWord = err == nil
		}
	})
	if o.isWord {
		len, err := Npd(c.wordOpe).parse(s, p+l, v, &context{s: s}, nil)
		if err != nil {
			return 0, err
		}
		l += len
	}

	// Skip whiltespace
	if c.inToken == false {
		if c.whitespaceOpe != nil {
			len, err := c.whitespaceOpe.parse(s, p+l, v, c, d)
			if err != nil {
				return 0, err
			}
			l += len
		}
	}
	return l, nil
}

func (o *literalString) accept(v visitor) {
	v.visitLiteralString(o)
}

// Character Class
type characterClass struct {
	opeBase
	chars string
}

func (o *characterClass) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	// TODO: UTF8 support
	if len(s)-p < 1 {
		c.setErrorPos(p)
		l = 0
		line, col := lineInfo(s, p)
		err = OperatorError{
			[]string{
				o.chars,
			},
			s,
			line,
			col,
			0,
		}

		return
	}
	ch := s[p]
	i := 0
	for i < len(o.chars) {
		if i+2 < len(o.chars) && o.chars[i+1] == '-' {
			if o.chars[i] <= ch && ch <= o.chars[i+2] {
				l = 1
				return
			}
			i += 3
		} else {
			if o.chars[i] == ch {
				l = 1
				return
			}
			i++
		}
	}
	c.setErrorPos(p)

	line, col := lineInfo(s, p)
	err = OperatorError{
		[]string{
			o.chars,
		},
		s,
		line,
		col,
		1,
	}
	return
}

func (o *characterClass) accept(v visitor) {
	v.visitCharacterClass(o)
}

// Any Character
type anyCharacter struct {
	opeBase
}

func (o *anyCharacter) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	// TODO: UTF8 support
	if len(s)-p < 1 {
		c.setErrorPos(p)
		l = 0
		line, col := lineInfo(s, p)
		err = OperatorError{
			[]string{
				"Anything",
			},
			s,
			line,
			col,
			0,
		}
		return
	}
	l = 1
	return
}

func (o *anyCharacter) accept(v visitor) {
	v.visitAnyCharacter(o)
}

// Token Boundary
type tokenBoundary struct {
	opeBase
	ope operator
}

func (o *tokenBoundary) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	c.inToken = true
	l, err := o.ope.parse(s, p, v, c, d)
	c.inToken = false
	if err == nil {
		v.Ts = append(v.Ts, Token{p, s[p : p+l]})

		// Skip whiltespace
		if c.whitespaceOpe != nil {
			len, err := c.whitespaceOpe.parse(s, p+l, v, c, d)
			if err != nil {
				return 0, err
			}
			l += len
		}
	}
	return l, err
}

func (o *tokenBoundary) accept(v visitor) {
	v.visitTokenBoundary(o)
}

// Ignore
type ignore struct {
	opeBase
	ope operator
}

func (o *ignore) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	chv := c.push()
	l, err := o.ope.parse(s, p, chv, c, d)
	c.pop()
	return l, err
}

func (o *ignore) accept(v visitor) {
	v.visitIgnore(o)
}

// User
type user struct {
	opeBase
	fn func(s string, p int, v *Values, d Any) (int, error)
}

func (o *user) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	return o.fn(s, p, v, d)
}

func (o *user) accept(v visitor) {
	v.visitUser(o)
}

// Reference
type reference struct {
	opeBase
	name  string
	iarg  int
	args  []operator
	iargs []int
	pos   int
	rule  *Rule
}

func (o *reference) parseCore(s string, p int, v *Values, c *context, d Any) (l int, err error) {
	if o.rule != nil {
		// Reference rule
		if o.rule.Parameters == nil {
			// Definition
			l, err = o.rule.parse(s, p, v, c, d)
		} else {
			// Macro
			vis := &findReference{
				args:   c.topArg(),
				params: o.rule.Parameters,
			}

			// Collect arguments
			var args []operator
			for _, arg := range o.args {
				arg.accept(vis)
				args = append(args, vis.ope)
			}

			c.pushArgs(args)
			l, err = o.rule.parse(s, p, v, c, d)
			c.popArgs()
		}
	} else {
		// Reference parameter in macro
		args := c.topArg()
		l, err = args[o.iarg].parse(s, p, v, c, d)
	}
	return
}

func (o *reference) accept(v visitor) {
	v.visitReference(o)
}

// Whitespace
type whitespace struct {
	opeBase
	ope operator
}

func (o *whitespace) parseCore(s string, p int, v *Values, c *context, d Any) (int, error) {
	if c.inWhitespace {
		return 0, nil
	} else {
		c.inWhitespace = true
		l, err := o.ope.parse(s, p, v, c, d)
		c.inWhitespace = false
		return l, err
	}
}

func (o *whitespace) accept(v visitor) {
	v.visitWhitespace(o)
}

func SeqCore(opes []operator) operator {
	o := &sequence{opes: opes}
	o.derived = o
	return o
}
func Seq(opes ...operator) operator {
	return SeqCore(opes)
}
func ChoCore(opes []operator) operator {
	o := &prioritizedChoice{opes: opes}
	o.derived = o
	return o
}
func Cho(opes ...operator) operator {
	return ChoCore(opes)
}
func Zom(ope operator) operator {
	o := &zeroOrMore{ope: ope}
	o.derived = o
	return o
}
func Oom(ope operator) operator {
	o := &oneOrMore{ope: ope}
	o.derived = o
	return o
}
func Opt(ope operator) operator {
	o := &option{ope: ope}
	o.derived = o
	return o
}
func Apd(ope operator) operator {
	o := &andPredicate{ope: ope}
	o.derived = o
	return o
}
func Npd(ope operator) operator {
	o := &notPredicate{ope: ope}
	o.derived = o
	return o
}
func Lit(lit string) operator {
	o := &literalString{lit: lit}
	o.derived = o
	return o
}
func Cls(chars string) operator {
	o := &characterClass{chars: chars}
	o.derived = o
	return o
}
func Dot() operator {
	o := &anyCharacter{}
	o.derived = o
	return o
}
func Tok(ope operator) operator {
	o := &tokenBoundary{ope: ope}
	o.derived = o
	return o
}
func Ign(ope operator) operator {
	o := &ignore{ope: ope}
	o.derived = o
	return o
}
func Usr(fn func(s string, p int, v *Values, d Any) (int, error)) operator {
	o := &user{fn: fn}
	o.derived = o
	return o
}
func Ref(ident string, args []operator, pos int) operator {
	o := &reference{name: ident, args: args, pos: pos}
	o.derived = o
	return o
}
func Wsp(ope operator) operator {
	o := &whitespace{ope: Ign(ope)}
	o.derived = o
	return o
}
