package pkg

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseBoxTemplate(t *testing.T) {
	template := `(var.yor_toggle ? /*<box>*/ { yor_trace = 123 } /*</box>*/ : {})`
	tokenTemplate, diagnostics := buildBoxFromTemplate(template)
	require.False(t, diagnostics.HasErrors())
	newFile := hclwrite.NewEmptyFile()
	newFile.Body().AppendUnstructuredTokens(tokenTemplate.Left)
	payload := hclwrite.Tokens{&hclwrite.Token{
		Type:         hclsyntax.TokenTemplateInterp,
		Bytes:        []byte("var.dummy"),
		SpacesBefore: 0,
	}}
	newFile.Body().AppendUnstructuredTokens(payload)
	newFile.Body().AppendUnstructuredTokens(tokenTemplate.Right)

	assertHclCodeEqual(t, `tags = (var.yor_toggle ? var.dummy : {})`, fmt.Sprintf("tags = %s", string(newFile.Bytes())))
}
