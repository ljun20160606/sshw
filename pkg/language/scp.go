package language

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/participle/lexer/ebnf"
	"strings"
)

var (
	scpLexer = lexer.Must(ebnf.New(`
	Ident = value { value } | "\"" { value } "\"" | "'" { value } "'" .
	Symbol = ":" | "@" .

    alpha = "a"…"z" | "A"…"Z" | "_" .
	number = "0"…"9" .
	value = alpha | number | "." | "-" | "/" | "\\" .
`))

	scpParser *participle.Parser
)

func init() {
	parser, err := participle.Build(&ScpDestinationGrammar{},
		participle.Lexer(scpLexer),
		participle.Map(func(t lexer.Token) (lexer.Token, error) {
			t.Value = strings.TrimFunc(t.Value, func(r rune) bool {
				return r == '\'' || r == '"'
			})
			return t, nil
		}, "Ident"),
	)
	if err != nil {
		panic(parser)
	}
	scpParser = parser
}

// 1. host:path
// 2. host:
// 3. user@host:path
// 4. user@host:
type ScpDestinationGrammar struct {
	User string `parser:"(@Ident \"@\")?"`
	Host string `parser:"@Ident \":\" "`
	Path string `parser:"@Ident?"`
}

func ParseScpDestination(input string) (*ScpDestinationGrammar, error) {
	ast := &ScpDestinationGrammar{}
	if err := scpParser.ParseString(input, ast); err != nil {
		return nil, err
	}
	return ast, nil
}
