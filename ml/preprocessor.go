package ml

import (
	"errors"
	"fmt"
	"sort"

	"cloudquant/market"
)

type DataPreprocessor struct {
	featureStats map[string][2]float64
}

func (p *DataPreprocessor) LoadHistoricalData(symbol string, days int) ([]MLFeatures, error) {
	if symbol == "" {
		return nil, errors.New("symbol is required")
	}
	if days <= 0 {
		return nil, errors.New("days must be positive")
	}
	klines, err := market.FetchHistoricalData(symbol, days)
	if err != nil {
		return nil, err
	}
	return ExtractFeatures(klines)
}

func (p *DataPreprocessor) ComputeStats(features []MLFeatures) error {
	if len(features) == 0 {
		return errors.New("features is empty")
	}
	p.featureStats = computeFeatureStats(features)
	return nil
}

func (p *DataPreprocessor) Normalize(features []MLFeatures) ([][]float64, error) {
	if len(features) == 0 {
		return nil, errors.New("features is empty")
	}
	if p.featureStats == nil {
		return nil, errors.New("feature stats not computed")
	}

	vectors := make([][]float64, len(features))
	names := FeatureNames()
	mins := make([]float64, len(names))
	maxs := make([]float64, len(names))
	for i, name := range names {
		stats, ok := p.featureStats[name]
		if !ok {
			return nil, fmt.Errorf("missing stats for %s", name)
		}
		mins[i] = stats[0]
		maxs[i] = stats[1]
	}

	for i, feature := range features {
		vector := FeatureVector(feature)
		normalized, err := NormalizeVector(vector, mins, maxs)
		if err != nil {
			return nil, err
		}
		vectors[i] = normalized
	}
	return vectors, nil
}

func (p *DataPreprocessor) FeatureStats() map[string][2]float64 {
	if p.featureStats == nil {
		return nil
	}
	copy := make(map[string][2]float64, len(p.featureStats))
	keys := make([]string, 0, len(p.featureStats))
	for key := range p.featureStats {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		copy[key] = p.featureStats[key]
	}
	return copy
}
