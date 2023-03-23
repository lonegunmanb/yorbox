package pkg

import (
	"strings"
	"testing"

	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoxFile(t *testing.T) {
	inputs := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "box resource",
			input: `
					resource "example_resource" "example_instance" {
		           name = "example"
		           tags = {
		               yor_trace = "example_trace"
		               environment = "dev"
		           }
			}
		`,
			expected: `
			resource "example_resource" "example_instance" {
		           name = "example"
		           tags = (var.yor_toggle ? {
		               yor_trace = "example_trace"
		               environment = "dev"
		           } : {})
			}
		`,
		},
		{
			name: "multiple yor tags",
			input: `
			resource "example_resource" "example_instance" {  
            name = "example"  
            tags = merge(each.value.tags, {}, (var.yor_toggle ? {
    			yor_trace = "0c9a0220-f447-473a-a142-0ed147c43691"
    			} : {}), {
   			 	git_commit           = "d101883be1a535645f359f1e1a047cf4b30bc2a2"
    			git_file             = "main.tf"
    			git_last_modified_at = "2023-03-23 11:09:02"
    			git_last_modified_by = "hezijie@microsoft.com"
    			git_modifiers        = "hezijie/lonegunmanb"
    			git_org              = "Azure"
    			git_repo             = "terraform-azurerm-aks"
  			})
	}  
`,
			expected: `
	resource "example_resource" "example_instance" {  
            name = "example"  
            tags = merge(each.value.tags, {}, (var.yor_toggle ? {
    			yor_trace = "0c9a0220-f447-473a-a142-0ed147c43691"
    			} : {}), (var.yor_toggle ? {
   			 	git_commit           = "d101883be1a535645f359f1e1a047cf4b30bc2a2"
    			git_file             = "main.tf"
    			git_last_modified_at = "2023-03-23 11:09:02"
    			git_last_modified_by = "hezijie@microsoft.com"
    			git_modifiers        = "hezijie/lonegunmanb"
    			git_org              = "Azure"
    			git_repo             = "terraform-azurerm-aks"
  			} : {}))
	}
`,
		},
		{
			name: "data should not be boxed",
			input: `
				data "example_data" "foo" {
		           name = "example"
		           tags = {
		               yor_trace = "example_trace"
		               environment = "dev"
		           }
				}
		`,
			expected: `
				data "example_data" "foo" {
		           name = "example"
		           tags = {
		               yor_trace = "example_trace"
		               environment = "dev"
		           }
				}
		`,
		},
		{
			name: "module should be boxed",
			input: `
				module "example_module" {
					source = "../../"
		           name = "example"
		           tags = {
		               yor_trace = "example_trace"
		               environment = "dev"
		           }
				}
		`,
			expected: `
				module "example_module" {
					source = "../../"
		           name = "example"
		           tags = (var.yor_toggle ? {
		               yor_trace = "example_trace"
		               environment = "dev"
		           } : {})
				}
		`,
		},
	}
	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		t.Run(input.name, func(t *testing.T) {
			file, diag := hclwrite.ParseConfig([]byte(input.input), "test.tf", hcl.InitialPos)
			require.False(t, diag.HasErrors())
			BoxFile(file, "yor_toggle")
			actual := string(file.Bytes())
			assertHclCodeEqual(t, input.expected, actual)
			file, diag = hclwrite.ParseConfig([]byte(actual), "test.tf", hcl.InitialPos)
			actual = string(file.Bytes())
			assertHclCodeEqual(t, input.expected, actual)
		})
	}
}

func TestScanYorTagsRanges_ValidResourceBlock(t *testing.T) {
	inputs := []struct {
		name string
		code string
		want []tokensRange
	}{
		{
			name: "single yor_trace tag",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = {  
                yor_trace = "example_trace"  
                environment = "dev"  
            }  
        }  
    `,
			want: []tokensRange{
				{Start: 2, End: 16},
			},
		},
		{
			name: "single yor_trace tag merge with other tags",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = merge({  
                yor_trace = "example_trace"
            }, {
				environment = "dev"  
			})
        }  
    `,
			want: []tokensRange{
				{Start: 4, End: 12},
			},
		},
		{
			name: "single yor_trace tag, json syntax",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = {  
                "yor_trace": "example_trace"  
                "environment": "dev"  
            }  
        }  
    `,
			want: []tokensRange{
				{Start: 2, End: 20},
			},
		},
		{
			name: "single yor_trace tag merge with other tags, different order",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = merge({  
				environment = "dev"
            }, {
				yor_trace = "example_trace"
			})
        }  
    `,
			want: []tokensRange{
				{Start: 14, End: 22},
			},
		},
		{
			name: "both yor_trace and git_commit in same block",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = {  
                yor_trace = "example_trace"  
                git_commit = "12345"  
            }  
        }  
    `,
			want: []tokensRange{
				{Start: 2, End: 16},
			},
		},
		{
			name: "yor_trace and git_commit in different block",
			code: `  
        resource "example_resource" "example_instance" {  
            name = "example"  
            tags = merge({  
                yor_trace = "example_trace"  
            }, {
				git_commit = "12345"  
			})
		}
    `,
			want: []tokensRange{
				{Start: 4, End: 12},
				{Start: 14, End: 22},
			},
		},
	}
	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		t.Run(input.name, func(t *testing.T) {
			// Parse the resource block into HCL tokens
			file, diags := hclwrite.ParseConfig([]byte(input.code), "", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatalf("Failed to parse resource block: %s", diags.Error())
			}

			// Get the tokens for the resource block
			tagsToken := getTagsTokens(file.Body().Blocks()[0])
			require.NotNil(t, tagsToken)

			// Call the scanYorTagsRanges function with the resource tokens
			yorTagsRanges := scanYorTagsRanges(tagsToken)

			assert.Equal(t, input.want, yorTagsRanges)
		})
	}
}

func TestScanForYorToggleRanges(t *testing.T) {
	inputs := []struct {
		name string
		code string
		want []tokensRange
	}{
		{
			name: "single_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge(var.tags, var.yor_toggle ? {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {})  
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: []tokensRange{
				{Start: 8, End: 11},
			},
		},
		{
			name: "multiple_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge(var.tags, var.yor_toggle ? {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {}, var.yor_toggle ? {
			  yor_trace			   = "12345"
			} : {} )  
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: []tokensRange{
				{Start: 8, End: 11},
				{Start: 25, End: 28},
			},
		},
		{
			name: "yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge(var.tags, {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			})  
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: []tokensRange{},
		},
	}

	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		t.Run(input.name, func(t *testing.T) {
			// Parse the resource block into HCL tokens
			file, diags := hclwrite.ParseConfig([]byte(input.code), "", hcl.InitialPos)
			if diags.HasErrors() {
				t.Fatalf("Failed to parse resource block: %s", diags.Error())
			}

			// Get the tokens for the resource block
			tokens := file.Body().Blocks()[0].Body().GetAttribute("tags").BuildTokens(hclwrite.Tokens{})
			require.NotNil(t, tokens)

			// Call the scanYorTagsRanges function with the resource tokens
			toggleRanges := scanYorToggleRanges(tokens, "yor_toggle")

			assert.Equal(t, input.want, toggleRanges)
		})
	}
}

func TestBoxYorTags(t *testing.T) {
	inputs := []struct {
		name string
		code string
		want string
	}{
		{
			name: "no_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = {  
			  env           = "app"  
			}
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = {  
			  env           = "app"  
			}
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
		},
		{
			name: "single_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			}
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = (var.yor_toggle ? {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {})
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
		},
		{
			name: "single_yor_toggle_with_merge",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge({
              env = "app"
			},{  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			})
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge({
              env = "app"
			}, (var.yor_toggle ? {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {}))
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
		},
		{
			name: "single_yor_toggle_json_style",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = {  
			  git_commit: "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			}
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = (var.yor_toggle ? {  
			  git_commit: "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {})
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
		},
		{
			name: "multiple_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge({  
			  git_commit= "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			}, {
			  env = "app"
			}, {
              yor_trace = "12345"
			})
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge((var.yor_toggle ? {  
			  git_commit= "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {}), {
			  env = "app"
			}, (var.yor_toggle ? {
              yor_trace = "12345"
			} : {}))
			workload_identity_enabled = var.workload_identity_enabled
		}	
	`,
		},
	}

	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		t.Run(input.name, func(t *testing.T) {
			// Parse the resource block into HCL tokens
			file, diags := hclwrite.ParseConfig([]byte(input.code), "", hcl.InitialPos)
			require.False(t, diags.HasErrors())

			boxTagsTokensForBlock(file.Body().Blocks()[0], "yor_toggle")
			boxedCode := string(file.Bytes())
			assertHclCodeEqual(t, input.want, boxedCode)
			boxedFile, diags := hclwrite.ParseConfig([]byte(boxedCode), "", hcl.InitialPos)
			require.False(t, diags.HasErrors())
			boxTagsTokensForBlock(boxedFile.Body().Blocks()[0], "yor_toggle")
			boxedCode = string(file.Bytes())
			assertHclCodeEqual(t, input.want, boxedCode)
		})
	}
}

func assertHclCodeEqual(t *testing.T, code1, code2 string) {
	config1, diag := hclwrite.ParseConfig([]byte(code1), "", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	config2, diag := hclwrite.ParseConfig([]byte(code2), "", hcl.InitialPos)
	require.False(t, diag.HasErrors())
	tokens1 := make([]hclwrite.Token, 0)
	tokens2 := make([]hclwrite.Token, 0)

	linq.From(config1.BuildTokens(hclwrite.Tokens{})).Where(func(i any) bool {
		return i.(*hclwrite.Token).Type != hclsyntax.TokenNewline
	}).Select(func(i interface{}) interface{} {
		t := *i.(*hclwrite.Token)
		t.SpacesBefore = 0
		return t
	}).ToSlice(&tokens1)
	linq.From(config2.BuildTokens(hclwrite.Tokens{})).Where(func(i any) bool {
		return i.(*hclwrite.Token).Type != hclsyntax.TokenNewline
	}).Select(func(i interface{}) interface{} {
		t := *i.(*hclwrite.Token)
		t.SpacesBefore = 0
		return t
	}).ToSlice(&tokens2)
	assert.Equal(t, tokens1, tokens2)
}

func trimCode(code string) string {
	code = strings.TrimSpace(code)
	code = strings.Trim(code, "\n")
	code = strings.Trim(code, "\t")
	return code
}

func getTagsTokens(block *hclwrite.Block) hclwrite.Tokens {
	tags := block.Body().GetAttribute("tags")
	if tags == nil {
		return nil
	}
	return tags.BuildTokens(hclwrite.Tokens{})
}
