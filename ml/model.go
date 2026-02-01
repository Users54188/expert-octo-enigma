package ml

type MLModel interface {
	Train(features [][]float64, labels []int) error
	Predict(features []float64) (int, float64, error)
	Save(path string) error
	Load(path string) error
}
