package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"encoding/csv"
	"os"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distuv"
	"flag"
)

// Network is a neural network with 3 layers
type Network struct {
	inputs       	int
	hiddens      	int
	outputs      	int
	learningRate 	float64
	hiddenWeights 	*mat.Dense
	outputWeights 	*mat.Dense
	hidden_max		float64
	hidden_min		float64
	out_max			float64
	out_min			float64
	score			int
}

// CreateNetwork creates a neural network with random weights
func CreateNetwork(input, hidden, output int, rate float64) (net Network) {
	net = Network{
		inputs:       input,
		hiddens:      hidden,
		outputs:      output,
		learningRate: rate,
	}
	net.hiddenWeights = mat.NewDense(net.hiddens, net.inputs, randomArray(net.inputs*net.hiddens, float64(net.inputs)))
	net.hidden_min = mat.Min(net.hiddenWeights)
	net.hidden_max = mat.Max(net.hiddenWeights)
	net.outputWeights = mat.NewDense(net.outputs, net.hiddens, randomArray(net.hiddens*net.outputs, float64(net.hiddens)))
	net.out_min = mat.Min(net.outputWeights)
	net.out_max = mat.Max(net.outputWeights)

	return
}

// Train the neural network
func (net *Network) Train(inputData []float64, targetData []float64) {
	// feedforward
	inputs := mat.NewDense(len(inputData), 1, inputData)
	hiddenInputs := dot(net.hiddenWeights, inputs)
	hiddenOutputs := apply(sigmoid, hiddenInputs)
	finalInputs := dot(net.outputWeights, hiddenOutputs)
	finalOutputs := apply(sigmoid, finalInputs)

	// find errors
	targets := mat.NewDense(len(targetData), 1, targetData)
	outputErrors := subtract(targets, finalOutputs)
	hiddenErrors := dot(net.outputWeights.T(), outputErrors)

	copy := mat.DenseCopyOf(outputErrors)
	copy.MulElem(copy, outputErrors)
	//net.errors = math.Sqrt(mat.Sum(copy))

	// backpropagate
	net.outputWeights = add(net.outputWeights,
		scale(net.learningRate,
			dot(multiply(outputErrors, sigmoidPrime(finalOutputs)),
				hiddenOutputs.T()))).(*mat.Dense)

	net.hiddenWeights = add(net.hiddenWeights,
		scale(net.learningRate,
			dot(multiply(hiddenErrors, sigmoidPrime(hiddenOutputs)),
				inputs.T()))).(*mat.Dense)
	truncateMatrix(net.outputWeights)
	truncateMatrix(net.hiddenWeights)
}

var numBits uint64
func initFlag() {
	flag.Uint64Var(&numBits, "truncation_bits", 24, "Number of bits truncated from mantissa")
}

func getBitMask(n uint64) uint64 {
	var bitMask uint64 = 0xffffffffffffffff<<n
	return bitMask
}

// Truncate digits of mat.Dense
func truncateMatrix(m *mat.Dense) {
	rows, columns := m.Dims()
	for r:=0; r<rows; r++ {
		for c:=0; c<columns; c++{
			m.Set(r, c, truncateM(m.At(r, c)))
		}
	}
}

/* Truncate the mantissa in float64
func truncateMantissa(a float64) float64{
	isNegative := a < 0
        if(isNegative){
                a = -a
        }
        i := uint64(math.Float64bits(a))
        mantissa := (i & 0xfffffffffffff)+0x10000000000000
	originalExponent := i>>52
	if originalExponent==0 {
		return 0
	}
	exponent := int64(originalExponent - 1023)
	if exponent>=0 {
		if exponent<11 {
			mantissa = mantissa<<exponent
		} else {
			fmt.Println("value too big")
			return 0
		}
	} else {
		mantissa = mantissa>>(-exponent)
	}
	// truncate numBits
	var bitMask uint64 = getBitMask(numBits)
	mantissa = mantissa & bitMask
	if exponent>=0 {
		mantissa = mantissa>>exponent
	} else {
		mantissa = mantissa<<(-exponent)
	}
	truncated := originalExponent<<52 + mantissa & 0xfffffffffffff
	if isNegative {
		truncated = 1<<63 + truncated
	}
	return math.Float64frombits(truncated)
}*/

func truncateM(a float64) float64{
	isNegative := a < 0
	if (isNegative) {
		a = -a
	}
	i := uint64(math.Float64bits(a))
	mantissa := (i & 0xfffffffffffff)+0x10000000000000
	originalExponent := i>>52
	if originalExponent==0 {
		return 0
	}
	exponent := int64(originalExponent) - 1023
	if exponent >= int64(numBits) {
		if isNegative {
			return -a
		}
		return a
	}
	realNumBits := uint64(int64(numBits) - exponent)
	var bitMask uint64 = getBitMask(realNumBits)
	mantissa = mantissa & bitMask
	truncated := originalExponent << 52 + mantissa & 0xfffffffffffff
	if isNegative {
		truncated = 1<<63 + truncated
	}
	// b := math.Float64frombits(truncated)
        // fmt.Println("original_exp: ", originalExponent, ", exp: ", exponent, ", realNumBits: ", realNumBits, "a: ", a, "b: ", b, ", negative: ", isNegative)
	return math.Float64frombits(truncated)
}

// Predict uses the neural network to predict the value given input data
func (net Network) Predict(inputData []float64) mat.Matrix {
	// feedforward
	inputs := mat.NewDense(len(inputData), 1, inputData)
	hiddenInputs := dot(net.hiddenWeights, inputs)
	truncateMatrix((hiddenInputs).(*mat.Dense))
	hiddenOutputs := apply(sigmoid, hiddenInputs)
	truncateMatrix((hiddenOutputs).(*mat.Dense))
	finalInputs := dot(net.outputWeights, hiddenOutputs)
	truncateMatrix((finalInputs).(*mat.Dense))
	finalOutputs := apply(sigmoid, finalInputs)
	truncateMatrix((finalOutputs).(*mat.Dense))
	return finalOutputs
}

func sigmoid(r, c int, z float64) float64 {
	return 1.0 / (1 + math.Exp(-1*z))
}

func sigmoidPrime(m mat.Matrix) mat.Matrix {
	rows, _ := m.Dims()
	o := make([]float64, rows)
	for i := range o {
		o[i] = 1
	}
	ones := mat.NewDense(rows, 1, o)
	return multiply(m, subtract(ones, m)) // m * (1 - m)
}

func relu(r, c int, z float64) float64 {
	if z>0 {
		return z
	}
	return 0
}

func relup(r, c int, z float64) float64 {
	if z>0 {
		return 1
	}
	return 0
}

func reluPrime(m mat.Matrix) mat.Matrix {
	return apply(relup, m)
}

//
// Helper functions to allow easier use of Gonum
//

func dot(m, n mat.Matrix) mat.Matrix {
	r, _ := m.Dims()
	_, c := n.Dims()
	o := mat.NewDense(r, c, nil)
	o.Product(m, n)
	//truncateMatrix(o)
	return o
}

func apply(fn func(i, j int, v float64) float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Apply(fn, m)
	//truncateMatrix(o)
	return o
}

func scale(s float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Scale(s, m)
	//truncateMatrix(o)
	return o
}

func multiply(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.MulElem(m, n)
	//truncateMatrix(o)
	return o
}

func add(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Add(m, n)
	//truncateMatrix(o)
	return o
}

func addScalar(i float64, m mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	a := make([]float64, r*c)
	for x := 0; x < r*c; x++ {
		a[x] = i
	}
	n := mat.NewDense(r, c, a)
	return add(m, n)
}

func subtract(m, n mat.Matrix) mat.Matrix {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	o.Sub(m, n)
	//truncateMatrix(o)
	return o
}

// randomly generate a float64 array
func randomArray(size int, v float64) (data []float64) {
	dist := distuv.Uniform{
		Min: -1 / math.Sqrt(v),
		Max: 1 / math.Sqrt(v),
	}

	data = make([]float64, size)
	for i := 0; i < size; i++ {
		// data[i] = rand.NormFloat64() * math.Pow(v, -0.5)
		data[i] = dist.Rand()
	}
	return
}

func addBiasNodeTo(m mat.Matrix, b float64) mat.Matrix {
	r, _ := m.Dims()
	a := mat.NewDense(r+1, 1, nil)

	a.Set(0, 0, b)
	for i := 0; i < r; i++ {
		a.Set(i+1, 0, m.At(i, 0))
	}
	return a
}

// pretty print a Gonum matrix
func matrixPrint(X mat.Matrix) {
	fa := mat.Formatted(X, mat.Prefix(""), mat.Squeeze())
	fmt.Printf("%v\n", fa)
}

func save(net Network) {
	h, err := os.Create("data/hweights.model")
	defer h.Close()
	if err == nil {
		net.hiddenWeights.MarshalBinaryTo(h)
	}
	o, err := os.Create("data/oweights.model")
	defer o.Close()
	if err == nil {
		net.outputWeights.MarshalBinaryTo(o)
	}
}

func save_plot(net Network, value [][]string) {
	file, _ := os.Create("data/plot.csv")
    defer file.Close()
    w := csv.NewWriter(file)
    defer w.Flush()
    w.WriteAll(value)
}

// load a neural network from file
func load(net *Network) {
	h, err := os.Open("data/hweights.model")
	defer h.Close()
	if err == nil {
		net.hiddenWeights.Reset()
		net.hiddenWeights.UnmarshalBinaryFrom(h)
	}
	o, err := os.Open("data/oweights.model")
	defer o.Close()
	if err == nil {
		net.outputWeights.Reset()
		net.outputWeights.UnmarshalBinaryFrom(o)
	}
	return
}

// predict a number from an image
// image should be 28 x 28 PNG file
func predictFromImage(net Network, path string) int {
	input := dataFromImage(path)
	output := net.Predict(input)
	matrixPrint(output)
	best := 0
	highest := 0.0
	for i := 0; i < net.outputs; i++ {
		if output.At(i, 0) > highest {
			best = i
			highest = output.At(i, 0)
		}
	}
	return best
}

// get the pixel data from an image
func dataFromImage(filePath string) (pixels []float64) {
	// read the file
	imgFile, err := os.Open(filePath)
	defer imgFile.Close()
	if err != nil {
		fmt.Println("Cannot read file:", err)
	}
	img, err := png.Decode(imgFile)
	if err != nil {
		fmt.Println("Cannot decode file:", err)
	}

	// create a grayscale image
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for x := 0; x < bounds.Max.X; x++ {
		for y := 0; y < bounds.Max.Y; y++ {
			var rgba = img.At(x, y)
			gray.Set(x, y, rgba)
		}
	}
	// make a pixel array
	pixels = make([]float64, len(gray.Pix))
	// populate the pixel array subtract Pix from 255 because that's how
	// the MNIST database was trained (in reverse)
	for i := 0; i < len(gray.Pix); i++ {
		pixels[i] = (float64(255-gray.Pix[i]) / 255.0 * 0.999) + 0.001
	}
	return
}

func max_weights(m, n mat.Matrix) *mat.Dense {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			o.Set(i, j, math.Max(m.At(i, j), n.At(i, j)))
		}
	} 
	//truncateMatrix(o)
	return o
}

func min_weights(m, n mat.Matrix) *mat.Dense {
	r, c := m.Dims()
	o := mat.NewDense(r, c, nil)
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			o.Set(i, j, math.Min(m.At(i, j), n.At(i, j)))
		}
	} 
	//truncateMatrix(o)
	return o
}
