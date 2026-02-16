package ml

import "context"

type MLModel interface {
	Train(features [][]float64, labels []int) error
	Predict(features []float64) (int, float64, error)
	Save(path string) error
	Load(path string) error
}

type ModelProvider interface {
	Predict(ctx context.Context, features map[string]float64) (interface{}, error)
}
