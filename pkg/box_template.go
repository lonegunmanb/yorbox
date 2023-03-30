package pkg

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type box struct {
	Left  hclwrite.Tokens
	Right hclwrite.Tokens
}

func interfaces(tokens hclwrite.Tokens) []any {
	result := make([]any, len(tokens))
	for i, token := range tokens {
		result[i] = token
	}
	return result
}

func buildBoxFromTemplate(template string) (box, hcl.Diagnostics) {
	template = fmt.Sprintf("tags = %s", template)
	f, diagnostics := hclwrite.ParseConfig([]byte(template), "", hcl.InitialPos)
	if diagnostics.HasErrors() {
		return box{}, diagnostics
	}
	templateTokens := f.Body().GetAttribute("tags").Expr().BuildTokens(hclwrite.Tokens{})
	leftTokens := hclwrite.Tokens{}
	rightTokens := hclwrite.Tokens{}
	inBox := false
	left := true

	for _, token := range templateTokens {
		if token.Type == hclsyntax.TokenComment {
			commentText := string(token.Bytes)
			if commentText == "/*<box>*/" {
				inBox = true
				left = false
				continue
			} else if commentText == "/*</box>*/" {
				inBox = false
				continue
			}
		}

		if !inBox {
			if left {
				leftTokens = append(leftTokens, token)
			} else {
				rightTokens = append(rightTokens, token)
			}
		}
	}

	return box{Left: leftTokens, Right: rightTokens}, hcl.Diagnostics{}
}
