package pkg

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/ahmetb/go-linq/v3"
	al "github.com/emirpasic/gods/lists/arraylist"
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
	Path        string
	ToggleName  string
	BoxTemplate string
	TagsPrefix  string
}

func NewOptions(path, toggleName, boxTemplate, tagsPrefix string) Options {
	if toggleName == "" {
		toggleName = "yor_toggle"
	}
	if boxTemplate == "" {
		boxTemplate = `/*<box>*/ (var.{{ .toggleName }} ? /*</box>*/ { yor_trace = 123 } /*<box>*/ : {}) /*</box>*/`
	}
	return Options{
		Path:        path,
		ToggleName:  toggleName,
		BoxTemplate: boxTemplate,
		TagsPrefix:  tagsPrefix,
	}
}

func (o Options) RenderBoxTemplate() (string, error) {
	return o.renderBoxTemplate(o.BoxTemplate)
}

func (o Options) renderBoxTemplate(tpl string) (string, error) {
	t := template.Must(template.New("Box").Funcs(sprig.TxtFuncMap()).Parse(tpl))
	vars := map[string]any{
		"dirPath":    o.Path,
		"toggleName": o.ToggleName,
		"tagsPrefix": o.TagsPrefix,
	}

	buff := &bytes.Buffer{}
	err := t.Execute(buff, vars)
	if err != nil {
		return "", err
	}
	return buff.String(), nil
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
		BoxFile(f, options)

		// Write the updated file contents back to the file
		err = os.WriteFile(filePath, f.Bytes(), os.ModePerm)
		if err != nil {
			panic(err.Error())
		}
	}
	return nil
}

func BoxFile(file *hclwrite.File, option Options) {
	for _, block := range file.Body().Blocks() {
		if block.Type() != "resource" && block.Type() != "module" {
			continue
		}
		boxTagsTokensForBlock(block, option)
	}
}

func boxTagsTokensForBlock(block *hclwrite.Block, option Options) {
	tags := block.Body().GetAttribute("tags")
	if tags == nil {
		return
	}

	tokens := tags.Expr().BuildTokens(hclwrite.Tokens{})
	tokens = removeYorToggles(tokens)
	output := al.New()
	for _, token := range tokens {
		output.Add(token)
	}
	tokensWithOutToggle := tokens
	yorTagsRanges := scanYorTagsRanges(tokensWithOutToggle, option)
	linq.From(yorTagsRanges).OrderByDescending(func(i interface{}) interface{} {
		return i.(tokensRange).End
	}).ToSlice(&yorTagsRanges)
	tplt, _ := option.RenderBoxTemplate()
	boxTemplate, _ := BuildBoxFromTemplate(tplt)
	for _, r := range yorTagsRanges {
		output.Insert(r.End+1, interfaces(boxTemplate.Right)...)
		output.Insert(r.Start, interfaces(boxTemplate.Left)...)
	}
	tokens = toTokens(output)
	//if tokens[0].Type != hclsyntax.TokenOParen || tokens[len(tokens)-1].Type != hclsyntax.TokenCParen {
	//	tokens = append(hclwrite.Tokens{
	//		&hclwrite.Token{
	//			Type:  hclsyntax.TokenOParen,
	//			Bytes: []byte("("),
	//		},
	//	}, tokens...)
	//	tokens = append(tokens, &hclwrite.Token{
	//		Type:  hclsyntax.TokenCParen,
	//		Bytes: []byte(")"),
	//	})
	//}
	block.Body().SetAttributeRaw("tags", tokens)
}

func exprWithLeadingComment(attr *hclwrite.Attribute) hclwrite.Tokens {
	tokens := attr.BuildTokens(hclwrite.Tokens{})
	equalIndex := -1
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type == hclsyntax.TokenEqual {
			equalIndex = i
			break
		}
	}
	return tokens[equalIndex+1:]
}

func scanYorTagsRanges(tokens hclwrite.Tokens, option Options) []tokensRange {
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
			previousYorTraceKey = name == fmt.Sprintf("%syor_name", option.TagsPrefix) ||
				name == fmt.Sprintf("%syor_trace", option.TagsPrefix) ||
				name == fmt.Sprintf("%sgit_commit", option.TagsPrefix)
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

func removeYorToggles(tokens hclwrite.Tokens) hclwrite.Tokens {
	result := hclwrite.Tokens{}
	inBox := false
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type == hclsyntax.TokenOParen && i < len(tokens)-1 && tokens[i+1].Type == hclsyntax.TokenComment {
			if string(tokens[i+1].Bytes) == "/*<box>*/" {
				inBox = true
				continue
			}
		}
		if tokens[i].Type == hclsyntax.TokenComment {
			if string(tokens[i].Bytes) == "/*<box>*/" {
				inBox = true
				continue
			} else if string(tokens[i].Bytes) == "/*</box>*/" {
				inBox = false
				if i < len(tokens)-1 && tokens[i+1].Type == hclsyntax.TokenCParen {
					i++
				}
				continue
			}
		}
		if !inBox {
			result = append(result, tokens[i])
		}
	}
	return result
}

func toTokens(l *al.List) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	it := l.Iterator()
	for it.Next() {
		tokens = append(tokens, it.Value().(*hclwrite.Token))
	}
	return tokens
}
