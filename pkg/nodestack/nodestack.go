package nodestack

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/google/go-jsonnet/ast"
	"github.com/sirupsen/logrus"
)

type NodeStack struct {
	From  ast.Node
	Stack []ast.Node
}

func NewNodeStack(from ast.Node) *NodeStack {
	return &NodeStack{
		From:  from,
		Stack: []ast.Node{from},
	}
}

func (s *NodeStack) Clone() *NodeStack {
	return &NodeStack{
		From:  s.From,
		Stack: append([]ast.Node{}, s.Stack...),
	}
}

func (s *NodeStack) Push(n ast.Node) {
	s.Stack = append(s.Stack, n)
}

func (s *NodeStack) PushNodes(nodes []ast.Node) {
	s.Stack = append(s.Stack, nodes...)
}

func (s *NodeStack) PushFront(n ast.Node) {
	s.Stack = append([]ast.Node{n}, s.Stack...)
}

func (s *NodeStack) Pop() ast.Node {
	l := len(s.Stack)
	if l == 0 {
		return nil
	}
	n := s.Stack[l-1]
	s.Stack = s.Stack[:l-1]
	return n
}

func (s *NodeStack) PopFront() ast.Node {
	l := len(s.Stack)
	if l == 0 {
		return nil
	}
	n := s.Stack[0]
	s.Stack = s.Stack[1:]
	return n
}

func (s *NodeStack) Peek() ast.Node {
	if len(s.Stack) == 0 {
		return nil
	}
	return s.Stack[len(s.Stack)-1]
}

func (s *NodeStack) PeekFront() ast.Node {
	if len(s.Stack) == 0 {
		return nil
	}
	return s.Stack[0]
}

func (s *NodeStack) IsEmpty() bool {
	return len(s.Stack) == 0
}

// RLY GO!? No member generics??? I thought you didn't have classes and this is just sugar for func(self, x, y)?!?
func (s *NodeStack) FindNext(nodeType reflect.Type) (ast.Node, int, error) {
	return s.FindNextFromIndex(nodeType, len(s.Stack)-1)
}

// Finds the next type going from index to 0
func (s *NodeStack) FindNextFromIndex(nodeType reflect.Type, index int) (ast.Node, int, error) {
	if len(s.Stack)-1 < index || index < 0 {
		return nil, 0, fmt.Errorf("invalid index %d. Max %d", index, len(s.Stack)-1)
	}
	// RLY GO? No proper iterators!?
	for i := index; i >= 0; i-- {
		if reflect.TypeOf(s.Stack[i]) == nodeType {
			return s.Stack[i], i, nil
		}
	}
	return nil, 0, fmt.Errorf("no node found")
}

func (s *NodeStack) BuildIndexList() []string {
	var indexList []string
	for !s.IsEmpty() {
		curr := s.Pop()
		switch curr := curr.(type) {
		case *ast.Apply:
			s.Push(curr.Target)
		case *ast.SuperIndex:
			s.Push(curr.Index)
			indexList = append(indexList, "super")
		case *ast.Index:
			s.Push(curr.Index)
			s.Push(curr.Target)
		case *ast.LiteralString:
			indexList = append(indexList, curr.Value)
		case *ast.Self:
			indexList = append(indexList, "self")
		case *ast.Var:
			indexList = append(indexList, string(curr.Id))
		case *ast.Import:
			indexList = append(indexList, curr.File.Value)
		}
	}
	return indexList
}

func (s *NodeStack) ReorderDesugaredObjects() *NodeStack {
	sort.SliceStable(s.Stack, func(i, j int) bool {
		_, iIsDesugared := s.Stack[i].(*ast.DesugaredObject)
		_, jIsDesugared := s.Stack[j].(*ast.DesugaredObject)
		if !iIsDesugared && !jIsDesugared {
			return false
		}

		iLoc, jLoc := s.Stack[i].Loc(), s.Stack[j].Loc()
		if iLoc.Begin.Line < jLoc.Begin.Line && iLoc.End.Line > jLoc.End.Line {
			return true
		}

		return false
	})
	return s
}

func (s *NodeStack) PrintStack() {
	tempstack := s.Clone()
	output := strings.Builder{}
	for !tempstack.IsEmpty() {
		currentNode := tempstack.Pop()
		output.WriteString(fmt.Sprintf("Stack: %T", currentNode))
		switch node := currentNode.(type) {
		case *ast.LiteralString:
			output.WriteString(fmt.Sprintf(" %s", node.Value))
		case *ast.Var:
			output.WriteString(fmt.Sprintf(" %s", node.Id))
		case *ast.Index:
			if nameNode, ok := node.Index.(*ast.LiteralString); ok {
				output.WriteString(fmt.Sprintf(" %s", nameNode.Value))
			}
		}
		output.WriteString("\n")
	}
	logrus.Errorf("%s", output.String())
}

func (s *NodeStack) GetNextFilename() string {
	filename := ""
	for i := len(s.Stack) - 1; i >= 0 && len(filename) == 0; i-- {
		filename = s.Stack[i].Loc().FileName
	}

	return filename
}
