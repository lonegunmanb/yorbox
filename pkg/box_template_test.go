package pkg

import (
	"fmt"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseBoxTemplate(t *testing.T) {
	template := `/*<box>*/(var.yor_toggle ? /*</box>*/ { yor_trace = 123 } /*<box>*/ : {})/*</box>*/`
	tokenTemplate, diagnostics := BuildBoxFromTemplate(template)
	require.False(t, diagnostics.HasErrors())
	newFile := hclwrite.NewEmptyFile()
	newFile.Body().AppendUnstructuredTokens(tokenTemplate.Left)
	payload := hclwrite.Tokens{&hclwrite.Token{
		Type:         hclsyntax.TokenTemplateInterp,
		Bytes:        []byte("var.dummy"),
		SpacesBefore: 1,
	}}
	newFile.Body().AppendUnstructuredTokens(payload)
	newFile.Body().AppendUnstructuredTokens(tokenTemplate.Right)

	actual := fmt.Sprintf("tags = %s", string(newFile.Bytes()))
	assert.Equal(t, formatHcl(t, `tags = (/*<box>*/ (var.yor_toggle ? /*</box>*/ var.dummy/*<box>*/ : {})/*</box>*/)`), formatHcl(t, actual))
}
