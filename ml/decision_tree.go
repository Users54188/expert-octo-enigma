package ml

import (
    "encoding/json"
    "errors"
    "math"
    "os"
)

type DecisionTree struct {
    nodes []TreeNode
}

type TreeNode struct {
    FeatureIdx int     `json:"feature_idx"`
    Threshold  float64 `json:"threshold"`
    LeftChild  int     `json:"left_child"`
    RightChild int     `json:"right_child"`
    ClassLabel int     `json:"class_label"`
    IsLeaf     bool    `json:"is_leaf"`
}

func (dt *DecisionTree) Train(features [][]float64, labels []int, maxDepth int) error {
    if len(features) == 0 || len(labels) == 0 {
        return errors.New("features or labels empty")
    }
    if len(features) != len(labels) {
        return errors.New("features and labels size mismatch")
    }
    if maxDepth <= 0 {
        maxDepth = 3
    }

    dt.nodes = nil
    nodes := dt.buildNode(features, labels, 0, maxDepth)
    dt.nodes = nodes
    return nil
}

func (dt *DecisionTree) Predict(features []float64) (int, float64, error) {
    if len(dt.nodes) == 0 {
        return 0, 0, errors.New("model not trained")
    }
    idx := 0
    for {
        node := dt.nodes[idx]
        if node.IsLeaf {
            return node.ClassLabel, nodeConfidence(idx), nil
        }
        if node.FeatureIdx < 0 || node.FeatureIdx >= len(features) {
            return 0, 0, errors.New("feature index out of range")
        }
        if features[node.FeatureIdx] <= node.Threshold {
            idx = node.LeftChild
        } else {
            idx = node.RightChild
        }
        if idx < 0 || idx >= len(dt.nodes) {
            return 0, 0, errors.New("invalid tree state")
        }
    }
}

func (dt *DecisionTree) Save(path string) error {
    if len(dt.nodes) == 0 {
        return errors.New("model not trained")
    }
    payload, err := json.Marshal(dt.nodes)
    if err != nil {
        return err
    }
    return os.WriteFile(path, payload, 0o600)
}

func (dt *DecisionTree) Load(path string) error {
    payload, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    var nodes []TreeNode
    if err := json.Unmarshal(payload, &nodes); err != nil {
        return err
    }
    dt.nodes = nodes
    return nil
}

func (dt *DecisionTree) buildNode(features [][]float64, labels []int, depth int, maxDepth int) []TreeNode {
    label := majorityLabel(labels)
    if depth >= maxDepth || isPure(labels) {
        return []TreeNode{{
            FeatureIdx: -1,
            Threshold:  0,
            LeftChild:  -1,
            RightChild: -1,
            ClassLabel: label,
            IsLeaf:     true,
        }}
    }

    bestFeature, threshold, ok := findBestSplit(features, labels)
    if !ok {
        return []TreeNode{{
            FeatureIdx: -1,
            Threshold:  0,
            LeftChild:  -1,
            RightChild: -1,
            ClassLabel: label,
            IsLeaf:     true,
        }}
    }

    leftFeatures, leftLabels, rightFeatures, rightLabels := splitData(features, labels, bestFeature, threshold)
    if len(leftLabels) == 0 || len(rightLabels) == 0 {
        return []TreeNode{{
            FeatureIdx: -1,
            Threshold:  0,
            LeftChild:  -1,
            RightChild: -1,
            ClassLabel: label,
            IsLeaf:     true,
        }}
    }

    leftNodes := dt.buildNode(leftFeatures, leftLabels, depth+1, maxDepth)
    rightNodes := dt.buildNode(rightFeatures, rightLabels, depth+1, maxDepth)

    root := TreeNode{
        FeatureIdx: bestFeature,
        Threshold:  threshold,
        LeftChild:  1,
        RightChild: 1 + len(leftNodes),
        ClassLabel: label,
        IsLeaf:     false,
    }

    nodes := make([]TreeNode, 0, 1+len(leftNodes)+len(rightNodes))
    nodes = append(nodes, root)
    nodes = append(nodes, leftNodes...)
    nodes = append(nodes, rightNodes...)
    return nodes
}

func findBestSplit(features [][]float64, labels []int) (int, float64, bool) {
    featureCount := len(features[0])
    bestFeature := -1
    bestThreshold := 0.0
    bestImpurity := math.MaxFloat64

    for featureIdx := 0; featureIdx < featureCount; featureIdx++ {
        values := make([]float64, len(features))
        for i := range features {
            values[i] = features[i][featureIdx]
        }
        threshold := median(values)
        leftLabels, rightLabels := splitLabels(features, labels, featureIdx, threshold)
        if len(leftLabels) == 0 || len(rightLabels) == 0 {
            continue
        }
        impurity := weightedGini(leftLabels, rightLabels)
        if impurity < bestImpurity {
            bestImpurity = impurity
            bestFeature = featureIdx
            bestThreshold = threshold
        }
    }
    if bestFeature == -1 {
        return -1, 0, false
    }
    return bestFeature, bestThreshold, true
}

func splitData(features [][]float64, labels []int, featureIdx int, threshold float64) ([][]float64, []int, [][]float64, []int) {
    leftFeatures := make([][]float64, 0)
    leftLabels := make([]int, 0)
    rightFeatures := make([][]float64, 0)
    rightLabels := make([]int, 0)
    for i, feature := range features {
        if feature[featureIdx] <= threshold {
            leftFeatures = append(leftFeatures, feature)
            leftLabels = append(leftLabels, labels[i])
        } else {
            rightFeatures = append(rightFeatures, feature)
            rightLabels = append(rightLabels, labels[i])
        }
    }
    return leftFeatures, leftLabels, rightFeatures, rightLabels
}

func splitLabels(features [][]float64, labels []int, featureIdx int, threshold float64) ([]int, []int) {
    leftLabels := make([]int, 0)
    rightLabels := make([]int, 0)
    for i, feature := range features {
        if feature[featureIdx] <= threshold {
            leftLabels = append(leftLabels, labels[i])
        } else {
            rightLabels = append(rightLabels, labels[i])
        }
    }
    return leftLabels, rightLabels
}

func weightedGini(leftLabels, rightLabels []int) float64 {
    leftWeight := float64(len(leftLabels))
    rightWeight := float64(len(rightLabels))
    total := leftWeight + rightWeight
    return (leftWeight/total)*gini(leftLabels) + (rightWeight/total)*gini(rightLabels)
}

func gini(labels []int) float64 {
    if len(labels) == 0 {
        return 0
    }
    counts := make(map[int]int)
    for _, label := range labels {
        counts[label]++
    }
    impurity := 1.0
    for _, count := range counts {
        prob := float64(count) / float64(len(labels))
        impurity -= prob * prob
    }
    return impurity
}

func median(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    sorted := append([]float64(nil), values...)
    sortFloats(sorted)
    mid := len(sorted) / 2
    if len(sorted)%2 == 0 {
        return (sorted[mid-1] + sorted[mid]) / 2
    }
    return sorted[mid]
}

func sortFloats(values []float64) {
    for i := 1; i < len(values); i++ {
        j := i
        for j > 0 && values[j-1] > values[j] {
            values[j-1], values[j] = values[j], values[j-1]
            j--
        }
    }
}

func majorityLabel(labels []int) int {
    counts := make(map[int]int)
    bestLabel := 0
    bestCount := -1
    for _, label := range labels {
        counts[label]++
        if counts[label] > bestCount {
            bestCount = counts[label]
            bestLabel = label
        }
    }
    return bestLabel
}

func isPure(labels []int) bool {
    if len(labels) == 0 {
        return true
    }
    first := labels[0]
    for _, label := range labels[1:] {
        if label != first {
            return false
        }
    }
    return true
}

func nodeConfidence(index int) float64 {
    if index < 0 {
        return 0
    }
    return 0.6
}
