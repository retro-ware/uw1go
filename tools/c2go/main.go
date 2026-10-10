// Command c2go translates the machine-generated C of Underworld Exhumed's port
// (src/port/**, written by Exhume's tools/asm2c.py from the matched assembly) into
// Go for package r3d. The C is a narrow dialect: one function per assembly module,
// global registers and flags, memory through rb/rw/wb/ww, goto labels. This tool
// parses that dialect and re-emits it with C's integer promotions made explicit.
//
// Usage: c2go -name INTERP in.c > out.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- lexer

type tok struct {
	k string // "id", "num", "str", "op", "eof"
	s string
}

func lex(src string) []tok {
	var ts []tok
	i := 0
	n := len(src)
	ops3 := []string{"<<=", ">>="}
	ops2 := []string{"<<", ">>", "<=", ">=", "==", "!=", "&&", "||", "++", "--", "+=", "-=", "&=", "|=", "^=", "->"}
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '/' && i+1 < n && src[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				i = n
			} else {
				i += j + 4
			}
		case c == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '#':
			for i < n && src[i] != '\n' {
				i++
			}
		case c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z':
			j := i
			for j < n && (src[j] == '_' || src[j] >= 'a' && src[j] <= 'z' || src[j] >= 'A' && src[j] <= 'Z' || src[j] >= '0' && src[j] <= '9') {
				j++
			}
			ts = append(ts, tok{"id", src[i:j]})
			i = j
		case c >= '0' && c <= '9':
			j := i
			for j < n && (src[j] >= '0' && src[j] <= '9' || src[j] >= 'a' && src[j] <= 'f' || src[j] >= 'A' && src[j] <= 'F' || src[j] == 'x' || src[j] == 'X' || src[j] == 'u' || src[j] == 'U' || src[j] == 'l' || src[j] == 'L') {
				j++
			}
			ts = append(ts, tok{"num", src[i:j]})
			i = j
		case c == '"':
			j := i + 1
			for j < n && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			ts = append(ts, tok{"str", src[i : j+1]})
			i = j + 1
		case c == '\'':
			j := i + 1
			for j < n && src[j] != '\'' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			v, _, _, err := strconv.UnquoteChar(src[i+1:j], '\'')
			if err != nil {
				panic(err)
			}
			ts = append(ts, tok{"num", strconv.Itoa(int(v))})
			i = j + 1
		default:
			matched := false
			for _, o := range ops3 {
				if strings.HasPrefix(src[i:], o) {
					ts = append(ts, tok{"op", o})
					i += 3
					matched = true
					break
				}
			}
			if !matched {
				for _, o := range ops2 {
					if strings.HasPrefix(src[i:], o) {
						ts = append(ts, tok{"op", o})
						i += 2
						matched = true
						break
					}
				}
			}
			if !matched {
				ts = append(ts, tok{"op", string(c)})
				i++
			}
		}
	}
	ts = append(ts, tok{"eof", ""})
	return ts
}

// ---------------------------------------------------------------- types

type ty int

const (
	tNone ty = iota
	tU8
	tI8
	tU16
	tI16
	tI32
	tU32
	tBool
	tBase // a memory base (segment pointer)
	tStr
)

var goName = map[ty]string{tU8: "uint8", tI8: "int8", tU16: "uint16", tI16: "int16", tI32: "int32", tU32: "uint32", tBool: "bool", tBase: "int"}

func ctype(s string) (ty, bool) {
	switch s {
	case "uint8_t":
		return tU8, true
	case "int8_t", "char":
		return tI8, true
	case "uint16_t":
		return tU16, true
	case "int16_t", "short":
		return tI16, true
	case "int", "int32_t":
		return tI32, true
	case "unsigned", "uint32_t":
		return tU32, true
	}
	return tNone, false
}

func size(t ty) int {
	switch t {
	case tU8, tI8:
		return 8
	case tU16, tI16:
		return 16
	}
	return 32
}

func signed(t ty) bool { return t == tI8 || t == tI16 || t == tI32 }

func promote1(t ty) ty {
	if t == tU32 {
		return tU32
	}
	return tI32
}

func promote(a, b ty) ty {
	if a == tU32 || b == tU32 {
		return tU32
	}
	return tI32
}

// ---------------------------------------------------------------- AST

type node struct {
	op   string // "num","id","call","cast","un","bin","tern","assign","post","pre","index","str"
	s    string
	val  int64
	t    ty // for cast
	kids []*node
}

type parser struct {
	ts  []tok
	p   int
	loc map[string]ty // locals
}

func (p *parser) peek() tok      { return p.ts[p.p] }
func (p *parser) peekN(n int) tok { return p.ts[p.p+n] }
func (p *parser) next() tok      { t := p.ts[p.p]; p.p++; return t }
func (p *parser) is(s string) bool {
	t := p.peek()
	return (t.k == "op" || t.k == "id") && t.s == s
}
func (p *parser) expect(s string) {
	t := p.next()
	if t.s != s {
		panic(fmt.Sprintf("expected %q, got %q at token %d (context %v)", s, t.s, p.p, p.ctx()))
	}
}
func (p *parser) ctx() string {
	var b []string
	for i := p.p - 8; i < p.p+8 && i < len(p.ts); i++ {
		if i >= 0 {
			b = append(b, p.ts[i].s)
		}
	}
	return strings.Join(b, " ")
}

func parseNum(s string) int64 {
	s = strings.TrimRight(s, "uUlL")
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		panic(err)
	}
	return v
}

var prec = map[string]int{
	"||": 1, "&&": 2, "|": 3, "^": 4, "&": 5, "==": 6, "!=": 6,
	"<": 7, ">": 7, "<=": 7, ">=": 7, "<<": 8, ">>": 8, "+": 9, "-": 9, "*": 10, "/": 10, "%": 10,
}

func (p *parser) expr() *node { return p.assign() }

func (p *parser) assign() *node {
	l := p.ternary()
	t := p.peek()
	if t.k == "op" && (t.s == "=" || t.s == "+=" || t.s == "-=" || t.s == "&=" || t.s == "|=" || t.s == "^=" || t.s == "<<=" || t.s == ">>=") {
		p.next()
		r := p.assign()
		return &node{op: "assign", s: t.s, kids: []*node{l, r}}
	}
	return l
}

func (p *parser) ternary() *node {
	c := p.binary(1)
	if p.is("?") {
		p.next()
		a := p.assign()
		p.expect(":")
		b := p.ternary()
		return &node{op: "tern", kids: []*node{c, a, b}}
	}
	return c
}

func (p *parser) binary(min int) *node {
	l := p.unary()
	for {
		t := p.peek()
		pr, ok := prec[t.s]
		if t.k != "op" || !ok || pr < min {
			return l
		}
		p.next()
		r := p.binary(pr + 1)
		l = &node{op: "bin", s: t.s, kids: []*node{l, r}}
	}
}

func (p *parser) isCast() (ty, bool) {
	if !p.is("(") {
		return tNone, false
	}
	t1 := p.peekN(1)
	if t1.k != "id" {
		return tNone, false
	}
	if t1.s == "const" {
		t1 = p.peekN(2)
	}
	tt, ok := ctype(t1.s)
	if !ok {
		return tNone, false
	}
	return tt, true
}

func (p *parser) unary() *node {
	t := p.peek()
	if t.k == "op" {
		switch t.s {
		case "-", "~", "!", "+":
			p.next()
			x := p.unary()
			return &node{op: "un", s: t.s, kids: []*node{x}}
		case "++", "--":
			p.next()
			x := p.unary()
			return &node{op: "pre", s: t.s, kids: []*node{x}}
		case "&":
			p.next()
			x := p.unary()
			return &node{op: "addr", kids: []*node{x}}
		}
	}
	if tt, ok := p.isCast(); ok {
		p.next()
		if p.is("const") {
			p.next()
		}
		p.next()
		for p.is("*") {
			panic("pointer cast at " + p.ctx())
		}
		p.expect(")")
		x := p.unary()
		return &node{op: "cast", t: tt, kids: []*node{x}}
	}
	return p.postfix()
}

func (p *parser) postfix() *node {
	x := p.primary()
	for {
		t := p.peek()
		if t.k == "op" && (t.s == "++" || t.s == "--") {
			p.next()
			x = &node{op: "post", s: t.s, kids: []*node{x}}
			continue
		}
		if t.k == "op" && t.s == "[" {
			p.next()
			i := p.expr()
			p.expect("]")
			x = &node{op: "index", kids: []*node{x, i}}
			continue
		}
		return x
	}
}

func (p *parser) primary() *node {
	t := p.next()
	switch t.k {
	case "num":
		return &node{op: "num", val: parseNum(t.s)}
	case "str":
		return &node{op: "str", s: t.s}
	case "id":
		if p.is("(") {
			p.next()
			var args []*node
			for !p.is(")") {
				args = append(args, p.assign())
				if p.is(",") {
					p.next()
				}
			}
			p.expect(")")
			return &node{op: "call", s: t.s, kids: args}
		}
		return &node{op: "id", s: t.s}
	case "op":
		if t.s == "(" {
			x := p.expr()
			p.expect(")")
			return x
		}
	}
	panic(fmt.Sprintf("unexpected %q in expression: %s", t.s, p.ctx()))
}

// ---------------------------------------------------------------- statements

type stmt struct {
	k     string // "block","if","while","for","goto","return","switch","label","expr","decl","break","continue","empty"
	label string
	e     *node
	t     ty
	name  string
	body  []*stmt
	els   []*stmt
	cases []caseArm
}

type caseArm struct {
	vals []int64
	def  bool
	body []*stmt
}

func (p *parser) stmtList(end string) []*stmt {
	var ss []*stmt
	for !p.is(end) {
		ss = append(ss, p.stmt()...)
	}
	return ss
}

func (p *parser) stmt() []*stmt {
	t := p.peek()
	if t.k == "id" && p.peekN(1).s == ":" && t.s != "default" {
		p.next()
		p.next()
		return []*stmt{{k: "label", label: t.s}}
	}
	if t.k == "op" && t.s == "{" {
		p.next()
		b := p.stmtList("}")
		p.expect("}")
		return []*stmt{{k: "block", body: b}}
	}
	if t.k == "op" && t.s == ";" {
		p.next()
		return []*stmt{{k: "empty"}}
	}
	if t.k == "id" {
		switch t.s {
		case "if":
			p.next()
			p.expect("(")
			c := p.expr()
			p.expect(")")
			body := p.stmt()
			s := &stmt{k: "if", e: c, body: body}
			if p.is("else") {
				p.next()
				s.els = p.stmt()
			}
			return []*stmt{s}
		case "while":
			p.next()
			p.expect("(")
			c := p.expr()
			p.expect(")")
			return []*stmt{{k: "while", e: c, body: p.stmt()}}
		case "for":
			p.next()
			p.expect("(")
			p.expect(";")
			p.expect(";")
			p.expect(")")
			return []*stmt{{k: "for", body: p.stmt()}}
		case "goto":
			p.next()
			l := p.next().s
			p.expect(";")
			return []*stmt{{k: "goto", label: l}}
		case "break", "continue":
			p.next()
			p.expect(";")
			return []*stmt{{k: t.s}}
		case "return":
			p.next()
			s := &stmt{k: "return"}
			if !p.is(";") {
				s.e = p.expr()
			}
			p.expect(";")
			return []*stmt{s}
		case "switch":
			p.next()
			p.expect("(")
			e := p.expr()
			p.expect(")")
			p.expect("{")
			s := &stmt{k: "switch", e: e}
			for !p.is("}") {
				var arm caseArm
				for p.is("case") || p.is("default") {
					if p.next().s == "default" {
						arm.def = true
					} else {
						arm.vals = append(arm.vals, constEval(p.ternary()))
					}
					p.expect(":")
				}
				for !p.is("case") && !p.is("default") && !p.is("}") {
					arm.body = append(arm.body, p.stmt()...)
				}
				s.cases = append(s.cases, arm)
			}
			p.expect("}")
			return []*stmt{s}
		}
		if t.s == "const" || t.s == "static" {
			p.next()
			return p.stmt()
		}
		if tt, ok := ctype(t.s); ok && p.peekN(1).k == "id" {
			p.next()
			var ss []*stmt
			for {
				name := p.next().s
				p.loc[name] = tt
				s := &stmt{k: "decl", t: tt, name: name}
				if p.is("=") {
					p.next()
					s.e = p.assign()
				}
				ss = append(ss, s)
				if p.is(",") {
					p.next()
					continue
				}
				break
			}
			p.expect(";")
			return ss
		}
	}
	e := p.expr()
	p.expect(";")
	return []*stmt{{k: "expr", e: e}}
}

// ---------------------------------------------------------------- constants

var namedConst = map[string]int64{
	"ASM_RET": 4, "ASM_RETF": 5, "ASM_IRET": 6, "PORT_LOAD_SEG": 0x800,
}

func isConst(e *node) bool {
	switch e.op {
	case "num":
		return true
	case "id":
		_, ok := namedConst[e.s]
		return ok
	case "un", "bin", "cast", "tern":
		for _, k := range e.kids {
			if !isConst(k) {
				return false
			}
		}
		return true
	}
	return false
}

func trunc(v int64, t ty) int64 {
	switch t {
	case tU8:
		return v & 0xFF
	case tI8:
		return int64(int8(v))
	case tU16:
		return v & 0xFFFF
	case tI16:
		return int64(int16(v))
	case tI32:
		return int64(int32(v))
	case tU32:
		return v & 0xFFFFFFFF
	}
	return v
}

func constEval(e *node) int64 {
	switch e.op {
	case "num":
		return e.val
	case "id":
		return namedConst[e.s]
	case "cast":
		return trunc(constEval(e.kids[0]), e.t)
	case "un":
		v := constEval(e.kids[0])
		switch e.s {
		case "-":
			return -v
		case "~":
			return ^v
		case "!":
			if v == 0 {
				return 1
			}
			return 0
		case "+":
			return v
		}
	case "tern":
		if constEval(e.kids[0]) != 0 {
			return constEval(e.kids[1])
		}
		return constEval(e.kids[2])
	case "bin":
		a, b := constEval(e.kids[0]), constEval(e.kids[1])
		b2 := func(x bool) int64 {
			if x {
				return 1
			}
			return 0
		}
		switch e.s {
		case "+":
			return a + b
		case "-":
			return a - b
		case "*":
			return a * b
		case "/":
			return a / b
		case "%":
			return a % b
		case "&":
			return a & b
		case "|":
			return a | b
		case "^":
			return a ^ b
		case "<<":
			return a << uint(b)
		case ">>":
			return a >> uint(b)
		case "==":
			return b2(a == b)
		case "!=":
			return b2(a != b)
		case "<":
			return b2(a < b)
		case ">":
			return b2(a > b)
		case "<=":
			return b2(a <= b)
		case ">=":
			return b2(a >= b)
		case "&&":
			return b2(a != 0 && b != 0)
		case "||":
			return b2(a != 0 || b != 0)
		}
	}
	panic("constEval " + e.op)
}

func lit(v int64, t ty) string {
	v = trunc(v, t)
	if v < 0 {
		return fmt.Sprintf("%d", v)
	}
	if v < 10 {
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("0x%X", v)
}

// ---------------------------------------------------------------- emitter

type sig struct {
	params []ty
	ret    ty
}

var funcs = map[string]sig{
	"rb": {[]ty{tBase, tU16}, tU8}, "rw": {[]ty{tBase, tU16}, tU16},
	"wb": {[]ty{tBase, tU16, tU8}, tNone}, "ww": {[]ty{tBase, tU16, tU16}, tNone},
	"push16": {[]ty{tU16}, tNone}, "pop16": {nil, tU16},
	"add16": {[]ty{tU16, tU16, tU32}, tU16}, "sub16": {[]ty{tU16, tU16, tU32}, tU16},
	"add8": {[]ty{tU8, tU8, tU32}, tU8}, "sub8": {[]ty{tU8, tU8, tU32}, tU8},
	"logic16": {[]ty{tU16}, tU16}, "logic8": {[]ty{tU8}, tU8},
	"inc16": {[]ty{tU16}, tU16}, "dec16": {[]ty{tU16}, tU16}, "neg16": {[]ty{tU16}, tU16},
	"inc8": {[]ty{tU8}, tU8}, "dec8": {[]ty{tU8}, tU8}, "neg8": {[]ty{tU8}, tU8},
	"shl16": {[]ty{tU16, tU32}, tU16}, "shr16": {[]ty{tU16, tU32}, tU16}, "sar16": {[]ty{tU16, tU32}, tU16},
	"rol16": {[]ty{tU16, tU32}, tU16}, "ror16": {[]ty{tU16, tU32}, tU16}, "rcl16": {[]ty{tU16, tU32}, tU16}, "rcr16": {[]ty{tU16, tU32}, tU16},
	"shl8": {[]ty{tU8, tU32}, tU8}, "shr8": {[]ty{tU8, tU32}, tU8}, "sar8": {[]ty{tU8, tU32}, tU8},
	"rol8": {[]ty{tU8, tU32}, tU8}, "ror8": {[]ty{tU8, tU32}, tU8}, "rcl8": {[]ty{tU8, tU32}, tU8}, "rcr8": {[]ty{tU8, tU32}, tU8},
	"shld16": {[]ty{tU16, tU16, tU32}, tU16}, "shrd16": {[]ty{tU16, tU16, tU32}, tU16},
	"mul8": {[]ty{tU8}, tNone}, "mul16": {[]ty{tU16}, tNone}, "imul8": {[]ty{tU8}, tNone}, "imul16": {[]ty{tU16}, tNone},
	"imul16x": {[]ty{tU16, tU16}, tU16},
	"asm_div8": {[]ty{tU8}, tBool}, "asm_idiv8": {[]ty{tU8}, tBool}, "asm_div16": {[]ty{tU16}, tBool}, "asm_idiv16": {[]ty{tU16}, tBool},
	"asm_divfault": {[]ty{tU16, tU16, tU32}, tU32},
	"asm_call":     {[]ty{tU32, tU16}, tU32}, "asm_callf": {[]ty{tU32, tU16, tU16}, tU32},
	"ASM_JMP":  {[]ty{tU16, tU16}, tU32},
	"asm_int":  {[]ty{tU8}, tU32},
	"asm_in8":  {[]ty{tU16}, tU8}, "asm_out8": {[]ty{tU16, tU8}, tNone}, "asm_out16": {[]ty{tU16, tU16}, tNone},
	"asm_jcc":  {[]ty{tU8}, tBool},
	"SET_DS":   {[]ty{tU16}, tNone}, "SET_ES": {[]ty{tU16}, tNone}, "SET_SS": {[]ty{tU16}, tNone},
	"asm_flags": {nil, tU16}, "asm_set_flags": {[]ty{tU16}, tNone},
	"port_halt": {[]ty{tStr}, tNone}, "asm_bad_entry": {[]ty{tStr, tU16}, tNone},
	"asm_halt_at":           {[]ty{tU16, tU16, tStr}, tNone},
	"scalebm_run_generated": {[]ty{tU16}, tNone},
	"palettesOff":           {nil, tU16}, "palettesSeg": {nil, tU16},
	"memEq":                 {[]ty{tBase, tBase, tI32}, tBool},
	"asm_glue_ret":          {nil, tU32}, "asm_glue_retf": {nil, tU32},
	"port_dgroup_seg":       {nil, tU16},
	"STEP":                  {[]ty{tI32}, tI32},
}

var regs16 = map[string]bool{"AX": true, "BX": true, "CX": true, "DX": true, "SI": true, "DI": true, "BP": true, "SP": true}
var regs8 = map[string]string{"AL": "AX", "AH": "AX", "BL": "BX", "BH": "BX", "CL": "CX", "CH": "CX", "DL": "DX", "DH": "DX"}
var globals = map[string]ty{
	"CF": tU8, "ZF": tU8, "SF": tU8, "OF": tU8, "DF": tU8,
	"asm_ds": tU16, "asm_es": tU16, "asm_ss": tU16,
	"pDS": tBase, "pES": tBase, "pSS": tBase,
	"CODE000": tBase, "CODE001": tBase, "CODE002": tBase, "CODE003": tBase, "CODE004": tBase, "CODE015": tBase, "CODE019": tBase,
	"c": tU32, "entry": tU16,
}

type emitter struct {
	loc  map[string]ty
	out  bytes.Buffer
	ind  int
	used map[string]bool
}

func (m *emitter) typeOf(e *node) ty {
	switch e.op {
	case "num":
		if e.val > 0x7FFFFFFF {
			return tU32
		}
		return tI32
	case "id":
		if _, ok := namedConst[e.s]; ok {
			return tI32
		}
		if regs16[e.s] {
			return tU16
		}
		if _, ok := regs8[e.s]; ok {
			return tU8
		}
		if t, ok := m.loc[e.s]; ok {
			return t
		}
		if t, ok := globals[e.s]; ok {
			return t
		}
		panic("unknown identifier " + e.s)
	case "call":
		s, ok := funcs[e.s]
		if !ok {
			panic("unknown function " + e.s)
		}
		return s.ret
	case "cast":
		return e.t
	case "index":
		return tU8
	case "un":
		if e.s == "!" {
			return tBool
		}
		return promote1(m.typeOf(e.kids[0]))
	case "bin":
		switch e.s {
		case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
			return tBool
		case "<<", ">>":
			return promote1(m.typeOf(e.kids[0]))
		}
		l, r := m.typeOf(e.kids[0]), m.typeOf(e.kids[1])
		if l == tBase || r == tBase {
			return tBase
		}
		return promote(l, r)
	case "tern":
		a, b := m.typeOf(e.kids[1]), m.typeOf(e.kids[2])
		if a == tBool && b == tBool {
			return tBool
		}
		return promote(a, b)
	case "assign", "post", "pre":
		return m.typeOf(e.kids[0])
	case "str":
		return tStr
	}
	panic("typeOf " + e.op)
}

// leaf emits an identifier read.
func (m *emitter) leaf(e *node) string {
	if r, ok := regs8[e.s]; ok {
		if e.s[1] == 'L' {
			return "uint8(" + r + ")"
		}
		return "uint8(" + r + " >> 8)"
	}
	return e.s
}

var wrapSafe = map[string]bool{"+": true, "-": true, "*": true, "&": true, "|": true, "^": true, "<<": true}

// as emits e converted to type t, with C's (t)e semantics.
func (m *emitter) as(e *node, t ty) string {
	if t == tBool {
		return m.cond(e)
	}
	if isConst(e) {
		return lit(constEval(e), t)
	}
	et := m.typeOf(e)
	if t == tBase {
		if et == tBase {
			return m.nat(e)
		}
		return "int(" + m.nat(e) + ")"
	}
	switch e.op {
	case "bin":
		if wrapSafe[e.s] && size(t) <= 32 && m.typeOf(e.kids[0]) != tBase {
			if e.s == "<<" {
				return "(" + m.as(e.kids[0], t) + " << " + m.shiftCount(e.kids[1]) + ")"
			}
			return "(" + m.as(e.kids[0], t) + " " + goOp(e.s) + " " + m.as(e.kids[1], t) + ")"
		}
	case "un":
		if e.s == "-" || e.s == "~" {
			op := e.s
			if op == "~" {
				op = "^"
			}
			return "(" + op + m.as(e.kids[0], t) + ")"
		}
		if e.s == "+" {
			return m.as(e.kids[0], t)
		}
	case "tern":
		return "ifv[" + goName[t] + "](" + m.cond(e.kids[0]) + ", " + m.as(e.kids[1], t) + ", " + m.as(e.kids[2], t) + ")"
	case "cast":
		if size(e.t) >= size(t) && et != tBool {
			// narrowing through a wider-or-equal cast: (t)(T)x == (t)x when T is at least t's size
			return m.as(e.kids[0], t)
		}
	}
	if et == t {
		return m.nat(e)
	}
	if et == tBool {
		return "b2i[" + goName[t] + "](" + m.nat(e) + ")"
	}
	return goName[t] + "(" + m.nat(e) + ")"
}

func goOp(s string) string {
	return s
}

func (m *emitter) shiftCount(e *node) string {
	if isConst(e) {
		return fmt.Sprintf("%d", constEval(e))
	}
	return "uint(" + m.as(e, tU32) + ")"
}

// cond emits e as a Go bool.
func (m *emitter) cond(e *node) string {
	if m.typeOf(e) == tBool {
		return m.nat(e)
	}
	if isConst(e) {
		if constEval(e) != 0 {
			return "true"
		}
		return "false"
	}
	return "(" + m.nat(e) + " != 0)"
}

// nat emits e in its natural (C-promoted) type, typeOf(e).
func (m *emitter) nat(e *node) string {
	if isConst(e) && e.op != "id" {
		return lit(constEval(e), m.typeOf(e))
	}
	switch e.op {
	case "num":
		return lit(e.val, m.typeOf(e))
	case "id":
		if v, ok := namedConst[e.s]; ok {
			return lit(v, tI32)
		}
		return m.leaf(e)
	case "str":
		return e.s
	case "call":
		s := funcs[e.s]
		var args []string
		if len(e.kids) != len(s.params) {
			panic(fmt.Sprintf("%s: %d args, want %d", e.s, len(e.kids), len(s.params)))
		}
		for i, k := range e.kids {
			if s.params[i] == tStr {
				args = append(args, k.s)
				continue
			}
			args = append(args, m.as(k, s.params[i]))
		}
		return goFunc(e.s) + "(" + strings.Join(args, ", ") + ")"
	case "cast":
		return m.as(e.kids[0], e.t)
	case "index":
		return "mem[" + m.as(e.kids[0], tBase) + "+int(" + m.nat(e.kids[1]) + ")]"
	case "un":
		switch e.s {
		case "!":
			return "!" + m.cond(e.kids[0])
		case "-":
			return "(-" + m.as(e.kids[0], m.typeOf(e)) + ")"
		case "~":
			return "(^" + m.as(e.kids[0], m.typeOf(e)) + ")"
		case "+":
			return m.as(e.kids[0], m.typeOf(e))
		}
	case "bin":
		switch e.s {
		case "&&", "||":
			return "(" + m.cond(e.kids[0]) + " " + e.s + " " + m.cond(e.kids[1]) + ")"
		case "==", "!=", "<", ">", "<=", ">=":
			lt, rt := m.typeOf(e.kids[0]), m.typeOf(e.kids[1])
			if lt == tBool && rt == tBool {
				return "(" + m.nat(e.kids[0]) + " " + e.s + " " + m.nat(e.kids[1]) + ")"
			}
			if lt == tBool {
				lt = tI32
			}
			if rt == tBool {
				rt = tI32
			}
			p := promote(lt, rt)
			// same-type unsigned or signed operands compare the same unpromoted
			if lt == rt {
				p = lt
			}
			return "(" + m.as(e.kids[0], p) + " " + e.s + " " + m.as(e.kids[1], p) + ")"
		case "<<", ">>":
			p := promote1(m.typeOf(e.kids[0]))
			return "(" + m.as(e.kids[0], p) + " " + e.s + " " + m.shiftCount(e.kids[1]) + ")"
		}
		t := m.typeOf(e)
		if t == tBase {
			return "(" + m.as(e.kids[0], tBase) + " " + e.s + " " + m.as(e.kids[1], tBase) + ")"
		}
		return "(" + m.as(e.kids[0], t) + " " + e.s + " " + m.as(e.kids[1], t) + ")"
	case "tern":
		t := m.typeOf(e)
		return "ifv[" + goName[t] + "](" + m.cond(e.kids[0]) + ", " + m.as(e.kids[1], t) + ", " + m.as(e.kids[2], t) + ")"
	}
	panic("nat " + e.op + " " + e.s)
}

func goFunc(s string) string {
	return s
}

// assignment to an lvalue: returns Go statement(s)
func (m *emitter) store(lv *node, val string) string {
	if lv.op != "id" {
		panic("lvalue " + lv.op)
	}
	if r, ok := regs8[lv.s]; ok {
		if lv.s[1] == 'L' {
			return fmt.Sprintf("%s = %s&0xFF00 | uint16(%s)", r, r, val)
		}
		return fmt.Sprintf("%s = %s&0x00FF | uint16(%s)<<8", r, r, val)
	}
	return lv.s + " = " + val
}

// exprStmt emits an expression evaluated for its effect.
func (m *emitter) exprStmt(e *node) []string {
	switch e.op {
	case "assign":
		lv := e.kids[0]
		t := m.typeOf(lv)
		rhs := e.kids[1]
		if e.s != "=" {
			rhs = &node{op: "bin", s: strings.TrimSuffix(e.s, "="), kids: []*node{lv, rhs}}
		}
		if rhs.op == "assign" {
			inner := m.exprStmt(rhs)
			return append(inner, m.store(lv, m.as(rhs.kids[0], t)))
		}
		return []string{m.store(lv, m.as(rhs, t))}
	case "post", "pre":
		lv := e.kids[0]
		op := "+"
		if e.s == "--" {
			op = "-"
		}
		return m.exprStmt(&node{op: "assign", s: "=", kids: []*node{lv, {op: "bin", s: op, kids: []*node{lv, {op: "num", val: 1}}}}})
	case "call":
		if funcs[e.s].ret == tNone || true {
			s := m.nat(e)
			if funcs[e.s].ret != tNone {
				return []string{"_ = " + s}
			}
			return []string{s}
		}
	case "bin":
		if e.s == "," {
			return append(m.exprStmt(e.kids[0]), m.exprStmt(e.kids[1])...)
		}
	case "cast":
		return m.exprStmt(e.kids[0])
	}
	panic("exprStmt " + e.op + " " + e.s)
}

func (m *emitter) line(s string) {
	m.out.WriteString(strings.Repeat("\t", m.ind))
	m.out.WriteString(s)
	m.out.WriteString("\n")
}

func hasAssign(e *node) bool {
	if e == nil {
		return false
	}
	if e.op == "assign" || e.op == "pre" || e.op == "post" {
		return true
	}
	for _, k := range e.kids {
		if hasAssign(k) {
			return true
		}
	}
	return false
}

// emitIf handles the conditions with side effects the generator writes:
// (c = X) != 0, A && (c = X) != 0, --R.
func (m *emitter) emitIf(s *stmt) {
	e := s.e
	if hasAssign(e) {
		switch {
		case e.op == "bin" && e.s == "&&" && !hasAssign(e.kids[0]):
			m.line("if " + m.cond(e.kids[0]) + " {")
			m.ind++
			m.emitIf(&stmt{k: "if", e: e.kids[1], body: s.body, els: nil})
			m.ind--
			m.line("}")
			if s.els != nil {
				panic("else with && assignment")
			}
			return
		case e.op == "bin" && (e.s == "!=" || e.s == "==") && e.kids[0].op == "assign" && !hasAssign(e.kids[1]):
			as := e.kids[0]
			for _, l := range m.exprStmt(as) {
				m.line(l)
			}
			ne := &node{op: "bin", s: e.s, kids: []*node{as.kids[0], e.kids[1]}}
			m.emitIf(&stmt{k: "if", e: ne, body: s.body, els: s.els})
			return
		case e.op == "bin" && e.s == "&&" && e.kids[0].op == "pre" && !hasAssign(e.kids[1]):
			for _, l := range m.exprStmt(e.kids[0]) {
				m.line(l)
			}
			ne := &node{op: "bin", s: "&&", kids: []*node{e.kids[0].kids[0], e.kids[1]}}
			m.emitIf(&stmt{k: "if", e: ne, body: s.body, els: s.els})
			return
		case e.op == "pre":
			for _, l := range m.exprStmt(e) {
				m.line(l)
			}
			m.emitIf(&stmt{k: "if", e: e.kids[0], body: s.body, els: s.els})
			return
		}
		panic("condition with side effects")
	}
	m.line("if " + m.cond(e) + " {")
	m.ind++
	m.stmts(s.body)
	m.ind--
	if s.els != nil {
		m.line("} else {")
		m.ind++
		m.stmts(s.els)
		m.ind--
	}
	m.line("}")
}

func (m *emitter) stmts(ss []*stmt) {
	for _, s := range ss {
		m.stmt1(s)
	}
}

func (m *emitter) stmt1(s *stmt) {
	switch s.k {
	case "label":
		if m.used[s.label] {
			m.ind--
			m.line(s.label + ":")
			m.ind++
		}
	case "block":
		m.line("{")
		m.ind++
		m.stmts(s.body)
		m.ind--
		m.line("}")
	case "empty":
	case "decl":
		if s.e != nil {
			m.line(fmt.Sprintf("var %s %s = %s", s.name, goName[s.t], m.as(s.e, s.t)))
		} else {
			m.line(fmt.Sprintf("var %s %s", s.name, goName[s.t]))
		}
		m.line("_ = " + s.name)
	case "if":
		m.emitIf(s)
	case "while":
		if hasAssign(s.e) {
			panic("while with side effects")
		}
		m.line("for " + m.cond(s.e) + " {")
		m.ind++
		m.stmts(s.body)
		m.ind--
		m.line("}")
	case "for":
		m.line("for {")
		m.ind++
		m.stmts(s.body)
		m.ind--
		m.line("}")
	case "goto":
		m.line("goto " + s.label)
	case "break", "continue":
		m.line(s.k)
	case "return":
		if s.e == nil {
			m.line("return")
		} else {
			m.line("return " + m.as(s.e, tU32))
		}
	case "expr":
		for _, l := range m.exprStmt(s.e) {
			m.line(l)
		}
	case "switch":
		m.line("switch " + m.nat(s.e) + " {")
		for _, c := range s.cases {
			if c.def {
				m.line("default:")
			} else {
				var vs []string
				for _, v := range c.vals {
					vs = append(vs, lit(v, m.typeOf(s.e)))
				}
				m.line("case " + strings.Join(vs, ", ") + ":")
			}
			m.ind++
			m.stmts(c.body)
			m.ind--
		}
		m.line("}")
	default:
		panic("stmt " + s.k)
	}
}

func collectGotos(ss []*stmt, used map[string]bool) {
	for _, s := range ss {
		if s.k == "goto" {
			used[s.label] = true
		}
		collectGotos(s.body, used)
		collectGotos(s.els, used)
		for _, c := range s.cases {
			collectGotos(c.body, used)
		}
	}
}

// ---------------------------------------------------------------- main

// fixups for the port's hand edits: the enhancements and test probes are off
// in this port, and the few C objects become runtime calls.
var fixups = []struct{ re, repl string }{
	{`\(void\)\w+;`, ``},
	{`(?m)^\s*uint32_t c;`, ``},
	{`persp_probe_next\(\);`, ``},
	{`if \(ENHANCED\([A-Z_]+\)\) \{[^\n]*\}\s*(/\*[^\n]*\*/)?\s*\n\s*else `, "\n    "},
	{`if \(ENHANCED\([A-Z_]+\)\) \{[^\n]*\}[^\n]*\n`, "\n"},
	{`persp_probe\(\) \? persp_probe_colour : `, ``},
	{`\(port_hole_mark\(\) >= 0 \? port_hole_mark\(\) \* 0x101 : 0\)`, `0`},
	{`port_fp_off\(Palettes\)`, `palettesOff()`},
	{`port_fp_seg\(Palettes\)`, `palettesSeg()`},
	{`memcmp\((CODE00\d) \+ (0x[0-9A-F]+), (CODE00\d) \+ (0x[0-9A-F]+), (0x[0-9A-F]+)\) == 0`, `memEq($1 + $2, $3 + $4, $5)`},
	{`(CODE00\d)\[(0x[0-9A-F]+)\]`, `rb($1, $2)`},
}

func main() {
	name := flag.String("name", "", "module name")
	flag.Parse()
	src, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	s := string(src)
	for _, f := range fixups {
		s = regexp.MustCompile(f.re).ReplaceAllString(s, f.repl)
	}
	// drop the per-file static asm_jcc (the runtime has it) and the externs
	s = regexp.MustCompile(`(?s)static int asm_jcc\(uint8_t op\)\n\{.*?\n\}\n`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?m)^(extern|void|int|uint16_t) [^\n(]*\([^\n]*\);[^\n]*$`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`(?m)^extern [^\n]*;[^\n]*$`).ReplaceAllString(s, "")
	start := regexp.MustCompile(`uint32_t asm_mod_\w+\(uint16_t entry\)\s*\{`).FindStringIndex(s)
	if start == nil {
		panic("no module function")
	}
	body := s[start[1]:]
	end := strings.LastIndex(body, "}")
	body = body[:end]

	p := &parser{ts: lex(body), loc: map[string]ty{}}
	var ss []*stmt
	for !p.eofOK() {
		ss = append(ss, p.stmt()...)
	}
	_ = ss
	m := &emitter{loc: p.loc, used: map[string]bool{}}
	collectGotos(ss, m.used)
	m.out.WriteString("// Code generated by tools/c2go from Underworld Exhumed's src/port translation of " + *name + ".ASM. DO NOT EDIT.\n\n")
	m.out.WriteString("package r3d\n\n")
	m.out.WriteString(fmt.Sprintf("func mod%s(entry uint16) uint32 {\n\tvar c uint32\n\t_ = c\n", *name))
	m.ind = 1
	// declarations inside the body are hoisted by Go's rules only within blocks; the
	// generator declares only inside braces, so emit as is
	m.stmts(ss)
	m.line("return 0")
	m.out.WriteString("}\n")
	os.Stdout.Write(m.out.Bytes())
}

func (p *parser) eofOK() bool { return p.peek().k == "eof" }

func init() {
	_ = (*parser).eofOK
}
