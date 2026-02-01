package ml

import (
	"errors"
)

func LoadModel(modelType, path string) (MLModel, error) {
	switch modelType {
	case "decision_tree":
		model := &DecisionTree{}
		if err := model.Load(path); err != nil {
			return nil, err
		}
		return model, nil
	default:
		return nil, errors.New("unsupported model type")
	}
}
