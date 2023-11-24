package main

import (
	"fmt"
	"os"
	"strings"

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

func patchFileAttributes(base *hclwrite.File, overlay *hclwrite.File) (*hclwrite.File, error) {
	patchBodyAttributes(base.Body(), overlay.Body())
	base.Body().AppendNewline()
	return base, nil
}

func patchBodyAttributes(base *hclwrite.Body, overlay *hclwrite.Body) (*hclwrite.Body, error) {
	overlayAttributes := overlay.Attributes()

	// use overlay attributes if they exist
	for name, overlayAttribute := range overlayAttributes {
		// Parse the attribute's tokens into an expression
		// filename is used only for diagnostic messages. so it can be placeholder string.
		expr, diags := hclsyntax.ParseExpression(overlayAttribute.Expr().BuildTokens(nil).Bytes(), "overlays", hcl.InitialPos)
		if diags.HasErrors() {
			return nil, diags
		}

		// Evaluate the expression to get a cty.Value
		val, diags := expr.Value(nil)
		if diags.HasErrors() {
			return nil, diags
		}

		base.SetAttributeValue(name, val)
	}

	return base, nil
}

func mergeFileBlocks(base *hclwrite.File, overlay *hclwrite.File) (*hclwrite.File, error) {
	mergeBlocks(base.Body(), overlay.Body())
	return base, nil
}

func mergeBlocks(base *hclwrite.Body, overlay *hclwrite.Body) (*hclwrite.Body, error) {
	baseBlocks := base.Blocks()
	overlayBlocks := overlay.Blocks()

	baseResourceBlocks := map[string]*hclwrite.Block{}
	baseDataBlocks := map[string]*hclwrite.Block{}

	for _, baseBlock := range baseBlocks {
		joinedLabel := strings.Join(baseBlock.Labels(), "_")
		switch baseBlock.Type() {
		case "resource":
			baseResourceBlocks[joinedLabel] = baseBlock
			base.RemoveBlock(baseBlock)
		case "data":
			baseDataBlocks[joinedLabel] = baseBlock
			base.RemoveBlock(baseBlock)
		case "locals":
			// TODO: handle locals
		default:
			// Handle other types
		}
	}

	// baseにあるblockをoverlayで上書き、なければ追加
	for _, overlayBlock := range overlayBlocks {
		joinedLabel := strings.Join(overlayBlock.Labels(), "_")
		switch overlayBlock.Type() {
		case "resource":
			if baseResourceBlock, ok := baseResourceBlocks[joinedLabel]; ok {
				mergedBlock, err := mergeBlock(baseResourceBlock, overlayBlock)
				if err != nil {
					return nil, err
				}
				baseResourceBlocks[joinedLabel] = mergedBlock
			} else {
				base.AppendBlock(overlayBlock)
			}
		case "data":
			if baseDataBlock, ok := baseDataBlocks[joinedLabel]; ok {
				mergedBlock, err := mergeBlock(baseDataBlock, overlayBlock)
				if err != nil {
					return nil, err
				}
				baseDataBlocks[joinedLabel] = mergedBlock
			} else {
				base.AppendBlock(overlayBlock)
			}
		case "locals":
			// TODO
		default:
			// Handle other types
		}
	}

	for _, baseResourceBlock := range baseResourceBlocks {
		base.AppendBlock(baseResourceBlock)
		base.AppendNewline()
	}
	for _, baseDataBlock := range baseDataBlocks {
		base.AppendBlock(baseDataBlock)
		base.AppendNewline()
	}

	return base, nil
}

func mergeBlock(baseBlock *hclwrite.Block, overlayBlock *hclwrite.Block) (*hclwrite.Block, error) {
	baseBlockBody := baseBlock.Body()
	overlayBlockBody := overlayBlock.Body()

	// どちらにも定義があるattributeをpatch
	patchBodyAttributes(baseBlockBody, overlayBlockBody)

	// overlay側にのみ定義があるattirbuteを追加
	// obtain and add attributes that are only defined in overlay
	overlayBodyAttributes := overlayBlockBody.Attributes()
	for name, overlayAttribute := range overlayBodyAttributes {
		if baseBlockBody.GetAttribute(name) == nil {
			baseBlockBody.SetAttributeRaw(name, overlayAttribute.Expr().BuildTokens(nil))
		}
	}

	// add blocks that are defined in overlay
	overlayBodyBlocks := overlayBlockBody.Blocks()
	for _, overlayBlock := range overlayBodyBlocks {
		baseBlockBody.AppendNewline()
		baseBlockBody.AppendBlock(overlayBlock)
	}

	return baseBlock, nil
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
	output, err := patchFileAttributes(base, overlay)
	if err != nil {
		panic(err)
	}
	output, err = mergeFileBlocks(base, overlay)

	fmt.Printf("%s", hclwrite.Format(output.Bytes()))
}
