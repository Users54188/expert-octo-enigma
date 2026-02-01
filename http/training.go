package http

import (
    "errors"
    "math"
    "math/rand"
    "os"
    "path/filepath"
    "time"

    "cloudquant/market"
    "cloudquant/ml"
)

type TrainingConfig struct {
    Symbol       string
    Days         int
    ModelType    string
    ModelPath    string
    MaxTreeDepth int
    TestRatio    float64
}

func trainModel(config TrainingConfig) error {
    if config.Symbol == "" {
        return errors.New("symbol is required")
    }
    if config.Days <= 0 {
        return errors.New("days must be positive")
    }
    if config.ModelPath == "" {
        return errors.New("model path is required")
    }

    preprocessor := &ml.DataPreprocessor{}
    features, err := preprocessor.LoadHistoricalData(config.Symbol, config.Days)
    if err != nil {
        return err
    }
    if len(features) == 0 {
        return errors.New("no features extracted")
    }
    klines, err := market.FetchHistoricalData(config.Symbol, config.Days)
    if err != nil {
        return err
    }
    labels, err := ml.GenerateLabels(klines, 3)
    if err != nil {
        return err
    }

    if len(labels) != len(features) {
        minLen := len(labels)
        if len(features) < minLen {
            minLen = len(features)
        }
        features = features[:minLen]
        labels = labels[:minLen]
    }

    featureVectors := make([][]float64, len(features))
    for i, feature := range features {
        featureVectors[i] = ml.FeatureVector(feature)
    }

    trainX, trainY, testX, testY := splitDataset(featureVectors, labels, config.TestRatio)

    model := &ml.DecisionTree{}
    if err := model.Train(trainX, trainY, config.MaxTreeDepth); err != nil {
        return err
    }

    if err := os.MkdirAll(filepath.Dir(config.ModelPath), 0o755); err != nil {
        return err
    }
    if err := model.Save(config.ModelPath); err != nil {
        return err
    }

    _, _ = testX, testY
    return nil
}

func splitDataset(features [][]float64, labels []int, testRatio float64) (trainX [][]float64, trainY []int, testX [][]float64, testY []int) {
    if testRatio <= 0 || testRatio >= 1 {
        testRatio = 0.2
    }
    rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
    indices := rnd.Perm(len(features))

    split := int(math.Round(float64(len(features)) * (1 - testRatio)))
    for i, idx := range indices {
        if i < split {
            trainX = append(trainX, features[idx])
            trainY = append(trainY, labels[idx])
        } else {
            testX = append(testX, features[idx])
            testY = append(testY, labels[idx])
        }
    }
    return trainX, trainY, testX, testY
}
