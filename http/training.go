package http

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"

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

	model := ml.NewDecisionTree(config.MaxTreeDepth)
	if err := model.Train(trainX, trainY, config.MaxTreeDepth); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(config.ModelPath), 0o750); err != nil {
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
	// 使用 crypto/rand 生成安全随机排列，替代弱随机数 math/rand
	indices := securePerm(len(features))

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

// securePerm 使用 crypto/rand 生成 0..n-1 的 Fisher-Yates 随机排列
// 替代弱随机数 math/rand.Perm，确保数据集划分不可预测
func securePerm(n int) []int {
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := secureIntn(i + 1)
		indices[i], indices[j] = indices[j], indices[i]
	}
	return indices
}

// secureIntn 使用 crypto/rand 生成 [0, max) 的均匀分布随机整数
func secureIntn(max int) int {
	if max <= 1 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}
	u := binary.BigEndian.Uint64(buf[:])
	// #nosec G115 -- max is small (<1000), overflow impossible in practice
	return int(u % uint64(max))
}
