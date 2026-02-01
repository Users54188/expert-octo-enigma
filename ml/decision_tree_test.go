package ml

import "testing"

func TestDecisionTreeTrainPredict(t *testing.T) {
	features := [][]float64{
		{0.1, 0.2},
		{0.2, 0.1},
		{0.9, 0.8},
		{0.8, 0.9},
	}
	labels := []int{0, 0, 2, 2}

	model := &DecisionTree{}
	if err := model.Train(features, labels, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	label, confidence, err := model.Predict([]float64{0.15, 0.15})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if label != 0 {
		t.Fatalf("expected label 0, got %d", label)
	}
	if confidence <= 0 {
		t.Fatalf("expected confidence > 0")
	}
}
