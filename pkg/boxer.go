package pkg

import (
	"os"
	"path/filepath"

	"github.com/ahmetb/go-linq/v3"
	al "github.com/emirpasic/gods/lists/arraylist"
	"github.com/emirpasic/gods/sets/hashset"
	lls "github.com/emirpasic/gods/stacks/linkedliststack"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type tokensRange struct {
	Start int
	End   int
}

type Options struct {
	Path       string
	ToggleName string
}

func ProcessDirectory(options Options) error {
	path := options.Path
	files, err := os.ReadDir(path)
	if err != nil {
		panic(err.Error())
	}

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".tf" {
			continue
		}

		filePath := filepath.Join(path, file.Name())

		// Read the file contents
		data, err := os.ReadFile(filePath)
		if err != nil {
			panic(err.Error())
		}

		// Parse the file to *hclwrite.File
		f, diag := hclwrite.ParseConfig(data, file.Name(), hcl.InitialPos)
		if diag.HasErrors() {
			return diag
		}

		// Invoke BoxFile function
		BoxFile(f, options.ToggleName)

		// Write the updated file contents back to the file
		err = os.WriteFile(filePath, f.Bytes(), os.ModePerm)
		if err != nil {
			panic(err.Error())
		}
	}
	return nil
}

func BoxFile(file *hclwrite.File, toggleName string) {
	for _, block := range file.Body().Blocks() {
		if block.Type() != "resource" && block.Type() != "module" {
			continue
		}
		boxTagsTokensForBlock(block, toggleName)
	}
}

func boxTagsTokensForBlock(block *hclwrite.Block, toggleName string) {
	// (var.yor_toggle ?
	var togglePrefixTokens = []any{
		&hclwrite.Token{
			Type:  hclsyntax.TokenOParen,
			Bytes: []byte("("),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte("var"),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenDot,
			Bytes: []byte("."),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenIdent,
			Bytes: []byte(toggleName),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenQuestion,
			Bytes: []byte("?"),
		},
	}

	// ": {})"
	var toggleSuffixTokens = []any{
		&hclwrite.Token{
			Type:  hclsyntax.TokenColon,
			Bytes: []byte(":"),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenOBrace,
			Bytes: []byte("{"),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenCBrace,
			Bytes: []byte("}"),
		},
		&hclwrite.Token{
			Type:  hclsyntax.TokenCParen,
			Bytes: []byte(")"),
		},
	}
	tags := block.Body().GetAttribute("tags")
	if tags == nil {
		return
	}
	tokens := tags.Expr().BuildTokens(hclwrite.Tokens{})
	output := al.New()
	for _, token := range tokens {
		output.Add(token)
	}
	toggleRanges := scanYorToggleRanges(tokens, toggleName)
	toggleTails := hashset.New()
	for _, r := range toggleRanges {
		toggleTails.Add(r.End + 1)
	}
	yorTagsRanges := scanYorTagsRanges(tokens)
	linq.From(yorTagsRanges).OrderByDescending(func(i interface{}) interface{} {
		return i.(tokensRange).End
	}).ToSlice(&yorTagsRanges)
	for _, r := range yorTagsRanges {
		if toggleTails.Contains(r.Start) {
			continue
		}
		output.Insert(r.End+1, toggleSuffixTokens...)
		output.Insert(r.Start, togglePrefixTokens...)
	}
	tokens = hclwrite.Tokens{}
	it := output.Iterator()
	for it.Next() {
		tokens = append(tokens, it.Value().(*hclwrite.Token))
	}
	block.Body().SetAttributeRaw("tags", tokens)
}

func scanYorTagsRanges(tokens hclwrite.Tokens) []tokensRange {
	ranges := make([]tokensRange, 0)
	latestOBrace := lls.New()
	var previousYorTraceKey bool
	yorTags := false
	for i, token := range tokens {
		switch token.Type {
		case hclsyntax.TokenNewline:
			continue
		case hclsyntax.TokenQuotedLit:
			fallthrough
		case hclsyntax.TokenIdent:
			name := string(token.Bytes)
			previousYorTraceKey = name == "yor_trace" || name == "git_commit"
		case hclsyntax.TokenEqual:
			fallthrough
		case hclsyntax.TokenColon:
			// we're sure `yor_trace =` or `git_commit =` or `yor_trace:` or `git_commit:`
			if previousYorTraceKey {
				yorTags = true
			}
		case hclsyntax.TokenOBrace:
			latestOBrace.Push(i)
		case hclsyntax.TokenCBrace:
			start, _ := latestOBrace.Pop()
			if yorTags {
				ranges = append(ranges, tokensRange{Start: start.(int), End: i})
				yorTags = false
			}
		default:
		}
	}
	return ranges
}

func scanYorToggleRanges(tokens hclwrite.Tokens, toggleName string) []tokensRange {
	ranges := make([]tokensRange, 0)
	for i, token := range tokens {
		if token.Type != hclsyntax.TokenIdent || string(token.Bytes) != "var" {
			continue
		}
		if i+3 >= len(tokens) {
			continue
		}
		if tokens[i+1].Type != hclsyntax.TokenDot {
			continue
		}
		if tokens[i+2].Type == hclsyntax.TokenIdent && string(tokens[i+2].Bytes) == toggleName {
			ranges = append(ranges, tokensRange{Start: i, End: i + 3})
		}
	}
	return ranges
}
