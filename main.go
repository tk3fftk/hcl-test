package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
)

func parse(filename string) (*hclwrite.File, error) {
	output := hclwrite.NewEmptyFile()

	src, err := os.ReadFile(filename)
	if err != nil {
		return output, err
	}

	file, diags := hclwrite.ParseConfig(src, filename, hcl.InitialPos)
	if diags.HasErrors() {
		return output, fmt.Errorf(diags.Error())
	}

	/*
		body := file.Body()
		for name, attribute := range body.Attributes() {
			fmt.Printf("%v, %v", name, attribute)
		}
		for _, block := range body.Blocks() {
			fmt.Println(block.Type())
			fmt.Println(block.Labels())
			fmt.Println(block.Body().Attributes())
			fmt.Println(block.Body().Blocks())
		}
	*/

	return file, nil
}

func patchAttributes(base *hclwrite.File, overlay *hclwrite.File) (*hclwrite.File, error) {
	overlayAttributes := overlay.Body().Attributes()

	// use overlay attributes if they exist
	for name, overlayAttribute := range overlayAttributes {
		// Parse the attribute's tokens into an expression
		expr, diags := hclsyntax.ParseExpression(overlayAttribute.Expr().BuildTokens(nil).Bytes(), "overlay.hcl", hcl.InitialPos)
		if diags.HasErrors() {
			return nil, diags
		}

		// Evaluate the expression to get a cty.Value
		val, diags := expr.Value(nil)
		if diags.HasErrors() {
			return nil, diags
		}

		base.Body().SetAttributeValue(name, val)
	}

	return base, nil
}

func main() {
	base, err := parse("base.hcl")
	if err != nil {
		panic(err)
	}
	overlay, err := parse("overlay.hcl")
	if err != nil {
		panic(err)
	}
	output, err := patchAttributes(base, overlay)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s", hclwrite.Format(output.Bytes()))
}
