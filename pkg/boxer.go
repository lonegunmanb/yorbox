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
	Path           string
	ToggleName     string
	BoxTemplate    string
	OldBoxTemplate string
	TagsPrefix     string
}

func NewOptions(path string, toggleName string, boxTemplate, oldTemplate string, tagsPrefix string) Options {
	if toggleName == "" {
		toggleName = "yor_toggle"
	}
	if boxTemplate == "" {
		boxTemplate = `(var.{{ .toggleName }} ? /*<box>*/ { yor_trace = 123 } /*</box>*/ : {})`
	}
	if oldTemplate == "" {
		oldTemplate = boxTemplate
	}
	return Options{
		Path:           path,
		ToggleName:     toggleName,
		BoxTemplate:    boxTemplate,
		OldBoxTemplate: oldTemplate,
		TagsPrefix:     tagsPrefix,
	}
}

func (o Options) RenderBoxTemplate() (string, error) {
	return o.renderBoxTemplate(o.BoxTemplate)
}

func (o Options) RenderOldBoxTemplate() (string, error) {
	return o.renderBoxTemplate(o.OldBoxTemplate)
}

func (o Options) renderBoxTemplate(tpl string) (string, error) {
	t := template.Must(template.New("box").Funcs(sprig.TxtFuncMap()).Parse(tpl))
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

func boxTagsTokensForBlock(block *hclwrite.Block, option Options) error {
	tplt, _ := option.RenderBoxTemplate()
	boxTemplate, _ := BuildBoxFromTemplate(tplt)
	oldTpl, _ := option.RenderOldBoxTemplate()

	tags := block.Body().GetAttribute("tags")
	if tags == nil {
		return nil
	}

	tokens := tags.Expr().BuildTokens(hclwrite.Tokens{})
	output := al.New()
	for _, token := range tokens {
		output.Add(token)
	}
	toggleRanges := scanYorToggleRanges(tokens, oldTpl)
	linq.From(toggleRanges).OrderByDescending(func(i interface{}) interface{} {
		return i.(tokensRange).End
	}).ToSlice(&toggleRanges)
	for _, r := range toggleRanges {
		removeRange(output, r.Start, r.End+1)
	}
	tokens = toTokens(output)
	yorTagsRanges := scanYorTagsRanges(tokens, option)
	linq.From(yorTagsRanges).OrderByDescending(func(i interface{}) interface{} {
		return i.(tokensRange).End
	}).ToSlice(&yorTagsRanges)
	for _, r := range yorTagsRanges {
		output.Insert(r.End+1, interfaces(boxTemplate.Right)...)
		output.Insert(r.Start, interfaces(boxTemplate.Left)...)
	}
	tokens = toTokens(output)
	block.Body().SetAttributeRaw("tags", tokens)
	return nil
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
			previousYorTraceKey = name == fmt.Sprintf("%syor_trace", option.TagsPrefix) || name == fmt.Sprintf("%sgit_commit", option.TagsPrefix)
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

func scanYorToggleRanges(tokens hclwrite.Tokens, boxTemplate string) []tokensRange {
	box, _ := BuildBoxFromTemplate(boxTemplate)
	ranges := make([]tokensRange, 0)
	for i, _ := range tokens {
		if i+len(box.Left) < len(tokens) && tokensEqual(tokens[i:i+len(box.Left)], box.Left) {
			ranges = append(ranges, tokensRange{Start: i, End: i + len(box.Left) - 1})
		}
		if i+len(box.Right) <= len(tokens) && tokensEqual(tokens[i:i+len(box.Right)], box.Right) {
			ranges = append(ranges, tokensRange{Start: i, End: i + len(box.Right) - 1})
		}
	}
	return ranges
}

func tokensEqual(tokens1, tokens2 hclwrite.Tokens) bool {
	if len(tokens1) != len(tokens2) {
		return false
	}
	for i := 0; i < len(tokens1); i++ {
		if tokens1[i].Type != tokens2[i].Type || string(tokens1[i].Bytes) != string(tokens2[i].Bytes) {
			return false
		}
	}
	return true
}

func removeRange(output *al.List, start int, end int) {
	for i := 0; i < end-start; i++ {
		output.Remove(start)
	}
}

func toTokens(l *al.List) hclwrite.Tokens {
	tokens := hclwrite.Tokens{}
	it := l.Iterator()
	for it.Next() {
		tokens = append(tokens, it.Value().(*hclwrite.Token))
	}
	return tokens
}
