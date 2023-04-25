package pkg

import (
	"fmt"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

type Box struct {
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

func BuildBoxFromTemplate(template string) (Box, hcl.Diagnostics) {
	template = fmt.Sprintf("tags = %s", template)
	f, diagnostics := hclwrite.ParseConfig([]byte(template), "", hcl.InitialPos)
	if diagnostics.HasErrors() {
		return Box{}, diagnostics
	}
	templateTokens := f.Body().GetAttribute("tags").BuildTokens(hclwrite.Tokens{})
	leftTokens := hclwrite.Tokens{}
	rightTokens := hclwrite.Tokens{}
	inBox := false
	left := true

	for _, token := range templateTokens {
		if token.Type == hclsyntax.TokenComment {
			commentText := string(token.Bytes)
			if commentText == "/*<box>*/" {
				inBox = true
			} else if commentText == "/*</box>*/" {
				inBox = false
				left = false
			}
		}

		if inBox {
			if left {
				leftTokens = append(leftTokens, token)
			} else {
				rightTokens = append(rightTokens, token)
			}
		}
	}
	endToken := &hclwrite.Token{
		Type:         hclsyntax.TokenComment,
		Bytes:        []byte("/*</box>*/"),
		SpacesBefore: 1,
	}
	leftTokens = append(leftTokens, endToken)
	rightTokens = append(rightTokens, endToken)

	return Box{Left: leftTokens, Right: rightTokens}, hcl.Diagnostics{}
}
