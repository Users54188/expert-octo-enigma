package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cloudquant/market"
	"cloudquant/ml"
)

func main() {
	symbol := flag.String("symbol", "", "stock symbol")
	days := flag.Int("days", 500, "number of days")
	modelPath := flag.String("model_path", "./models/dt.model", "model output path")
	maxDepth := flag.Int("max_depth", 10, "max tree depth")
	testRatio := flag.Float64("test_ratio", 0.2, "test ratio")
	flag.Parse()

	if *symbol == "" {
		log.Fatal("symbol is required")
	}

	features, labels, err := buildTrainingData(*symbol, *days)
	if err != nil {
		log.Fatalf("failed to build training data: %v", err)
	}

	trainX, trainY, testX, testY := splitDataset(features, labels, *testRatio)

	model := ml.NewDecisionTree(*maxDepth)
	if err := model.Train(trainX, trainY); err != nil {
		log.Fatalf("failed to train model: %v", err)
	}

	accuracy, precision, recall := evaluateModel(model, testX, testY)
	log.Printf("accuracy=%.2f precision=%.2f recall=%.2f", accuracy, precision, recall)

	if err := os.MkdirAll(filepath.Dir(*modelPath), 0o755); err != nil {
		log.Fatalf("failed to create model dir: %v", err)
	}
	if err := model.Save(*modelPath); err != nil {
		log.Fatalf("failed to save model: %v", err)
	}

	fmt.Printf("model saved to %s\n", *modelPath)
}

func buildTrainingData(symbol string, days int) ([][]float64, []int, error) {
	klines, err := market.FetchHistoricalData(symbol, days)
	if err != nil {
		return nil, nil, err
	}
	features, err := ml.ExtractFeatures(klines)
	if err != nil {
		return nil, nil, err
	}
	labels, err := ml.GenerateLabels(klines, 3)
	if err != nil {
		return nil, nil, err
	}

	featureVectors := make([][]float64, 0, len(features))
	labelValues := make([]int, 0, len(features))
	offset := len(klines) - len(features)
	for i, feature := range features {
		idx := i + offset
		if idx >= len(labels) {
			break
		}
		featureVectors = append(featureVectors, ml.FeatureVector(feature))
		labelValues = append(labelValues, labels[idx])
	}
	return featureVectors, labelValues, nil
}

func splitDataset(features [][]float64, labels []int, testRatio float64) (trainX [][]float64, trainY []int, testX [][]float64, testY []int) {
	if testRatio <= 0 || testRatio >= 1 {
		testRatio = 0.2
	}

	split := int(float64(len(features)) * (1 - testRatio))
	for i := range features {
		if i < split {
			trainX = append(trainX, features[i])
			trainY = append(trainY, labels[i])
		} else {
			testX = append(testX, features[i])
			testY = append(testY, labels[i])
		}
	}
	return trainX, trainY, testX, testY
}

func evaluateModel(model *ml.DecisionTree, testX [][]float64, testY []int) (accuracy, precision, recall float64) {
	if len(testX) == 0 {
		return 0, 0, 0
	}

	var correct int
	var truePositive int
	var predictedPositive int
	var actualPositive int

	for i, feature := range testX {
		label, _, err := model.Predict(feature)
		if err != nil {
			continue
		}
		if label == testY[i] {
			correct++
		}
		if label == 2 {
			predictedPositive++
		}
		if testY[i] == 2 {
			actualPositive++
			if label == 2 {
				truePositive++
			}
		}
	}

	accuracy = float64(correct) / float64(len(testX))
	if predictedPositive > 0 {
		precision = float64(truePositive) / float64(predictedPositive)
	}
	if actualPositive > 0 {
		recall = float64(truePositive) / float64(actualPositive)
	}
	return accuracy, precision, recall
}
