package processing

// func TestCompile(t *testing.T) {
// vm := jsonnet.MakeVM()
//
// processor := NewProcessor(cache.New(), vm)
// uri := protocol.URIFromPath("./testdata/varcompile.jsonnet")
// root, _, err := processor.vm.ImportAST("", uri.SpanURI().Filename())
//
// require.NoError(t, err)
//
// tree := nodetree.BuildTree(nil, root)
// logrus.Errorf("\n%s", tree)
//
// assert.Len(t, tree.Children, 2)
// tree = tree.Children[0]
// assert.Len(t, tree.Children, 2)
// tree = tree.Children[0]
// assert.Len(t, tree.Children, 2)
// tree = tree.Children[0]
// assert.Len(t, tree.Children, 3)
//
// funcNode := tree.Children[1].Node
// assert.Equal(t, reflect.TypeFor[*ast.Apply](), reflect.TypeOf(funcNode))
// _, err = processor.CompileNode(root, funcNode)
// require.NoError(t, err)
// }
