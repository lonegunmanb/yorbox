package pkg

import (
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
			BoxFile(file, NewOptions("", "yor_toggle", "", "", ""))
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
			yorTagsRanges := scanYorTagsRanges(tagsToken, NewOptions("", "", "", "", ""))

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
				tags = merge(var.tags, (var.yor_toggle ? {
				  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"
				} : {}))
				workload_identity_enabled = var.workload_identity_enabled
			}
		`,
			want: []tokensRange{
				{Start: 8, End: 12},
				{Start: 22, End: 25},
			},
		},
		{
			name: "multiple_yor_toggle",
			code: `  
		resource "azurerm_kubernetes_cluster" "main" {  
			tags = merge(var.tags, (var.yor_toggle ? {  
			  git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"  
			} : {}), (var.yor_toggle ? {
			  yor_trace			   = "12345"
			} : {}))  
			workload_identity_enabled = var.workload_identity_enabled  
		}  
	`,
			want: []tokensRange{
				{Start: 8, End: 12},
				{Start: 22, End: 25},
				{Start: 27, End: 31},
				{Start: 41, End: 44},
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
			options := NewOptions("", "yor_toggle", "", "", "")
			tplt, err := options.RenderBoxTemplate()
			require.NoError(t, err)
			options.BoxTemplate = tplt
			toggleRanges := scanYorToggleRanges(tokens, options.BoxTemplate)

			assert.Equal(t, input.want, toggleRanges)
		})
	}
}

func TestBoxYorTags(t *testing.T) {
	inputs := []struct {
		name        string
		code        string
		want        string
		boxTemplate string
		toggleName  string
		tagsPrefix  string
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
		{
			name: "yor_toggle_with_customized_box_template",
			code: `
			resource "azurerm_kubernetes_cluster" "main" {
				tags = {
				  my_tags_git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"
				}
				workload_identity_enabled = var.workload_identity_enabled
			}
		`,
			want: `
			resource "azurerm_kubernetes_cluster" "main" {
				tags = (var.my_toggle ? { for k, v in {
				  my_tags_git_commit           = "bb858b143c94abf2d08c88de77a0054ff5f85db5"
				} : replace(k, "my_tags_", var.yor_toggle_prefix) => v } : {})
				workload_identity_enabled = var.workload_identity_enabled
			}
		`,
			boxTemplate: `(var.{{ .toggleName }} ? { for k, v in /*<box>*/ { yor_trace = 123 } /*</box>*/ : replace(k, "{{ .tagsPrefix }}", var.yor_toggle_prefix) => v } : {})`,
			toggleName:  "my_toggle",
			tagsPrefix:  "my_tags_",
		},
		{
			name: "yor_toggle_with_customized_box_template2",
			code: `  
resource "azurerm_log_analytics_solution" "main" {
  count = local.create_analytics_solution ? 1 : 0

  location              = coalesce(var.location, data.azurerm_resource_group.main.location)
  resource_group_name   = coalesce(var.log_analytics_workspace_resource_group_name, var.resource_group_name)
  solution_name         = "ContainerInsights"
  workspace_name        = local.log_analytics_workspace.name
  workspace_resource_id = local.log_analytics_workspace.id
  tags = merge(var.tags, (var.yor_toggle ? { for k, v in {
    git_commit           = "e3016f23f676fcd2c1b07dd49a22f975d1616ab6"
    git_file             = "main.tf"
    git_last_modified_at = "2022-09-30 12:36:26"
    git_last_modified_by = "gissur@skyvafnir.is"
    git_modifiers        = "56525716+yupwei68/amit.gera/gissur/hezijie/lukasz.r.szczesniak"
    git_org              = "Azure"
    git_repo             = "terraform-azurerm-aks"
    yor_trace            = "a33efee5-b36e-4574-97b8-02fd9b7f2f6b"
  }  : "my_prefix_${k}" => v } : {}))

  plan {
    product   = "OMSGallery/ContainerInsights"
    publisher = "Microsoft"
  }
}
	`,
			want: `  
resource "azurerm_log_analytics_solution" "main" {
  count = local.create_analytics_solution ? 1 : 0

  location              = coalesce(var.location, data.azurerm_resource_group.main.location)
  resource_group_name   = coalesce(var.log_analytics_workspace_resource_group_name, var.resource_group_name)
  solution_name         = "ContainerInsights"
  workspace_name        = local.log_analytics_workspace.name
  workspace_resource_id = local.log_analytics_workspace.id
  tags = merge(var.tags, (var.yor_toggle ? { for k, v in {
    git_commit           = "e3016f23f676fcd2c1b07dd49a22f975d1616ab6"
    git_file             = "main.tf"
    git_last_modified_at = "2022-09-30 12:36:26"
    git_last_modified_by = "gissur@skyvafnir.is"
    git_modifiers        = "56525716+yupwei68/amit.gera/gissur/hezijie/lukasz.r.szczesniak"
    git_org              = "Azure"
    git_repo             = "terraform-azurerm-aks"
    yor_trace            = "a33efee5-b36e-4574-97b8-02fd9b7f2f6b"
  }  : "my_prefix_${k}" => v } : {}))

  plan {
    product   = "OMSGallery/ContainerInsights"
    publisher = "Microsoft"
  }
}
	`,
			boxTemplate: `(var.{{ .toggleName }} ? { for k, v in /*<box>*/ { yor_trace = 123 } /*</box>*/ : "my_prefix_${k}" => v } : {})`,
			toggleName:  "yor_toggle",
		},
	}

	for i := 0; i < len(inputs); i++ {
		input := inputs[i]
		t.Run(input.name, func(t *testing.T) {
			// Parse the resource block into HCL tokens
			file, diags := hclwrite.ParseConfig([]byte(input.code), "", hcl.InitialPos)
			require.False(t, diags.HasErrors())

			toggleName := "yor_toggle"
			if input.toggleName != "" {
				toggleName = input.toggleName
			}
			options := NewOptions("", toggleName, input.boxTemplate, "", input.tagsPrefix)
			boxTagsTokensForBlock(file.Body().Blocks()[0], options)
			boxedCode := string(file.Bytes())
			assertHclCodeEqual(t, input.want, boxedCode)
			boxedFile, diags := hclwrite.ParseConfig([]byte(boxedCode), "", hcl.InitialPos)
			require.False(t, diags.HasErrors())
			boxTagsTokensForBlock(boxedFile.Body().Blocks()[0], options)
			boxedCode = string(file.Bytes())
			assertHclCodeEqual(t, input.want, boxedCode)
		})
	}
}

func TestRenderBoxTemplateWithToggleName(t *testing.T) {
	template := `(var.{{ .toggleName }} ? /*<box>*/ { yor_trace = 123 } /*</box>*/ : {})`
	opt := NewOptions("", "my_toggle", template, "", "")
	tplt, err := opt.RenderBoxTemplate()
	require.NoError(t, err)
	assert.Equal(t, `(var.my_toggle ? /*<box>*/ { yor_trace = 123 } /*</box>*/ : {})`, tplt)
}

func TestChangingBoxTemplate(t *testing.T) {
	code := `resource "example_resource" "example_instance" {  
            name = "example"  
            tags = (var.yor_toggle ? {
    			yor_trace = "0c9a0220-f447-473a-a142-0ed147c43691"
    			} : {})
	}  
`
	file, diags := hclwrite.ParseConfig([]byte(code), "", hcl.InitialPos)
	require.False(t, diags.HasErrors())

	toggleName := "yor_toggle"
	oldTemplate := `(var.{{ .toggleName }} ? /*<box>*/ { yor_trace = 123 } /*</box>*/ : {})`
	newTemplate := `(var.{{ .toggleName }} ? { for k, v in /*<box>*/ { yor_trace = 123 } /*</box>*/ : "my_prefix_${k}" => v } : {})`
	options := NewOptions("", toggleName, newTemplate, oldTemplate, "")
	boxTagsTokensForBlock(file.Body().Blocks()[0], options)
	boxedCode := string(file.Bytes())
	expected := `resource "example_resource" "example_instance" {  
            name = "example"  
            tags = (var.yor_toggle ? { for k, v in {
    			yor_trace = "0c9a0220-f447-473a-a142-0ed147c43691"
    			} : "my_prefix_${k}" => v } : {})
	}  
`
	assert.Equal(t, formatHcl(expected), formatHcl(boxedCode))
}

func formatHcl(input string) string {
	// Create a new HCL file from the input string
	f, _ := hclwrite.ParseConfig([]byte(input), "", hcl.InitialPos)

	// Format the HCL file
	formatted := f.Bytes()

	return string(formatted)
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

func getTagsTokens(block *hclwrite.Block) hclwrite.Tokens {
	tags := block.Body().GetAttribute("tags")
	if tags == nil {
		return nil
	}
	return tags.BuildTokens(hclwrite.Tokens{})
}
