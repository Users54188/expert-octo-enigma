package ml

type MLModel interface {
	Train(features [][]float64, labels []int, maxDepth int) error
	Predict(features []float64) (int, float64, error)
	Save(path string) error
	Load(path string) error
}

// ModelProvider is an alias for MLModel, used by strategy layer
type ModelProvider = MLModel
