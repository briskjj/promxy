package parser

import "github.com/prometheus/prometheus/storage"

func NewRawMatrixFromVector(ms *VectorSelector) *RawMatrix {
	return &RawMatrix{
		Series: ms.Series,
	}
}

func NewRawMatrixFromMatrix(ms *MatrixSelector) *RawMatrix {
	return &RawMatrix{
		Series: ms.VectorSelector.(*VectorSelector).Series,
	}
}

// RawMatrix is a Value that the promql engine evaluates as a matrix regardless of context
// This is used to replace SubqueryExpr as they return a Matrix of data that isn't a MatrixSelector
type RawMatrix struct {
	Series []storage.Series
}

func (*RawMatrix) expr()             {}
func (e *RawMatrix) Type() ValueType { return ValueTypeMatrix }
func (e *RawMatrix) String() string {
	return "RAW SERIES" // TODO
}

func (e *RawMatrix) PositionRange() PositionRange {
	return PositionRange{}
}
