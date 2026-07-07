package transformer

import (
	"math"
	"math/rand"
)

// --- Matrix helpers (row-major float64 slices) ---

// matmul computes C = A @ B where A is [m x k], B is [k x n].
func matmul(A [][]float64, B [][]float64) [][]float64 {
	m := len(A)
	k := len(B)
	n := len(B[0])
	C := make([][]float64, m)
	for i := range C {
		C[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			for l := 0; l < k; l++ {
				C[i][j] += A[i][l] * B[l][j]
			}
		}
	}
	return C
}

// transpose returns the transpose of matrix A [m x n] → [n x m].
func transpose(A [][]float64) [][]float64 {
	m := len(A)
	n := len(A[0])
	T := make([][]float64, n)
	for i := range T {
		T[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			T[i][j] = A[j][i]
		}
	}
	return T
}

// softmax applies softmax row-wise.
func softmax(A [][]float64) [][]float64 {
	out := make([][]float64, len(A))
	for i, row := range A {
		out[i] = softmax1D(row)
	}
	return out
}

func softmax1D(v []float64) []float64 {
	max := v[0]
	for _, x := range v {
		if x > max {
			max = x
		}
	}
	out := make([]float64, len(v))
	sum := 0.0
	for j, x := range v {
		out[j] = math.Exp(x - max)
		sum += out[j]
	}
	for j := range out {
		out[j] /= sum
	}
	return out
}

// layerNorm applies layer normalization along the last dimension.
// Input: [seqLen x dim]  Output: [seqLen x dim]
func layerNorm(X [][]float64, eps float64) [][]float64 {
	out := make([][]float64, len(X))
	for i, row := range X {
		mean := 0.0
		for _, v := range row {
			mean += v
		}
		mean /= float64(len(row))

		variance := 0.0
		for _, v := range row {
			d := v - mean
			variance += d * d
		}
		variance /= float64(len(row))
		std := math.Sqrt(variance + eps)

		normed := make([]float64, len(row))
		for j, v := range row {
			normed[j] = (v - mean) / std
		}
		out[i] = normed
	}
	return out
}

// relu applies ReLU element-wise.
func relu(X [][]float64) [][]float64 {
	out := make([][]float64, len(X))
	for i, row := range X {
		r := make([]float64, len(row))
		for j, v := range row {
			if v > 0 {
				r[j] = v
			}
		}
		out[i] = r
	}
	return out
}

// add adds two matrices element-wise (residual connection).
func add(A, B [][]float64) [][]float64 {
	out := make([][]float64, len(A))
	for i := range A {
		row := make([]float64, len(A[i]))
		for j := range A[i] {
			row[j] = A[i][j] + B[i][j]
		}
		out[i] = row
	}
	return out
}

// --- Weight initialization ---

func randMatrix(rows, cols int) [][]float64 {
	m := make([][]float64, rows)
	scale := math.Sqrt(2.0 / float64(rows+cols))
	for i := range m {
		m[i] = make([]float64, cols)
		for j := range m[i] {
			m[i][j] = rand.NormFloat64() * scale
		}
	}
	return m
}

func randVec(n int) []float64 {
	v := make([]float64, n)
	for i := range v {
		v[i] = rand.NormFloat64() * 0.02
	}
	return v
}

// meanAbsPerToken returns the mean absolute activation per token position.
// Input: [seqLen x dim] → [seqLen]
func meanAbsPerToken(X [][]float64) []float64 {
	out := make([]float64, len(X))
	for i, row := range X {
		sum := 0.0
		for _, v := range row {
			sum += math.Abs(v)
		}
		out[i] = sum / float64(len(row))
	}
	return out
}
