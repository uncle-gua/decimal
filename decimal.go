package decimal

// release under the terms of file license.txt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Decimal is a fixed precision 38.24 number (supports 11.7 digits). It supports NaN.
type Decimal struct {
	fp int64
}

// the following constants can be changed to configure a different number of decimal places - these are
// the only required changes. only 18 significant digits are supported due to NaN

const nPlaces = 7
const scale = int64(10 * 10 * 10 * 10 * 10 * 10 * 10)
const zeros = "0000000"
const MAX = float64(99999999999.9999999)

const nan = int64(1<<63 - 1)

var NaN = Decimal{fp: nan}
var ZERO = Decimal{fp: 0}

var errTooLarge = errors.New("significand too large")
var errFormat = errors.New("invalid encoding")

// NewS creates a new Decimal from a string, returning NaN if the string could not be parsed
func NewS(s string) Decimal {
	f, _ := NewSErr(s)
	return f
}

// NewSErr creates a new Decimal from a string, returning NaN, and error if the string could not be parsed
func NewSErr(s string) (Decimal, error) {
	if strings.ContainsAny(s, "eE") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return NaN, err
		}
		return NewF(f), nil
	}
	if s == "NaN" {
		return NaN, nil
	}
	period := strings.Index(s, ".")
	var i int64
	var f int64
	var sign int64 = 1
	var err error
	if period == -1 {
		i, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return NaN, errors.New("cannot parse")
		}
		if i < 0 {
			sign = -1
			i = i * -1
		}
	} else {
		if len(s[:period]) > 0 {
			i, err = strconv.ParseInt(s[:period], 10, 64)
			if err != nil {
				return NaN, errors.New("cannot parse")
			}
			if i < 0 || s[0] == '-' {
				sign = -1
				i = i * -1
			}
		}
		fs := s[period+1:]
		fs = fs + zeros[:max(0, nPlaces-len(fs))]
		f, err = strconv.ParseInt(fs[0:nPlaces], 10, 64)
		if err != nil {
			return NaN, errors.New("cannot parse")
		}
	}
	if float64(i) > MAX {
		return NaN, errTooLarge
	}
	return Decimal{fp: sign * (i*scale + f)}, nil
}

// Parse creates a new Decimal from a string, returning NaN, and error if the string could not be parsed. Same as NewSErr
// but more standard naming
func Parse(s string) (Decimal, error) {
	return NewSErr(s)
}

// MustParse creates a new Decimal from a string, and panics if the string could not be parsed
func MustParse(s string) Decimal {
	f, err := NewSErr(s)
	if err != nil {
		panic(err)
	}
	return f
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// NewF creates a Decimal from an float64, rounding at the 8th decimal place
func NewF(f float64) Decimal {
	if math.IsNaN(f) {
		return Decimal{fp: nan}
	}
	if f >= MAX || f <= -MAX {
		return NaN
	}
	round := .5
	if f < 0 {
		round = -0.5
	}

	return Decimal{fp: int64(f*float64(scale) + round)}
}

// NewI creates a Decimal for an integer, moving the decimal point n places to the left
// For example, NewI(123,1) becomes 12.3. If n > 7, the value is truncated
func NewI(i int64, n uint) Decimal {
	if n > nPlaces {
		i = i / int64(math.Pow10(int(n-nPlaces)))
		n = nPlaces
	}

	i = i * int64(math.Pow10(int(nPlaces-n)))

	return Decimal{fp: i}
}

func (f Decimal) IsNaN() bool {
	return f.fp == nan
}

func (f Decimal) IsZero() bool {
	return f.Equal(ZERO)
}

// Sign returns:
//
//	-1 if f <  0
//	 0 if f == 0 or NaN
//	+1 if f >  0
func (f Decimal) Sign() int {
	if f.IsNaN() {
		return 0
	}
	return f.Cmp(ZERO)
}

// Float converts the Decimal to a float64
func (f Decimal) Float() float64 {
	if f.IsNaN() {
		return math.NaN()
	}
	return float64(f.fp) / float64(scale)
}

// Add adds f0 to f producing a Decimal. If either operand is NaN, NaN is returned
func (f Decimal) Add(f0 Decimal) Decimal {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return Decimal{fp: f.fp + f0.fp}
}

// Sub subtracts f0 from f producing a Decimal. If either operand is NaN, NaN is returned
func (f Decimal) Sub(f0 Decimal) Decimal {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return Decimal{fp: f.fp - f0.fp}
}

// Abs returns the absolute value of f. If f is NaN, NaN is returned
func (f Decimal) Abs() Decimal {
	if f.IsNaN() {
		return NaN
	}
	if f.Sign() >= 0 {
		return f
	}
	f0 := Decimal{fp: f.fp * -1}
	return f0
}

func abs(i int64) int64 {
	if i >= 0 {
		return i
	}
	return i * -1
}

// Mul multiplies f by f0 returning a Decimal. If either operand is NaN, NaN is returned
func (f Decimal) Mul(f0 Decimal) Decimal {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}

	fp_a := f.fp / scale
	fp_b := f.fp % scale

	fp0_a := f0.fp / scale
	fp0_b := f0.fp % scale

	var result int64

	if fp0_a != 0 {
		result = fp_a*fp0_a*scale + fp_b*fp0_a
	}
	if fp0_b != 0 {
		result = result + (fp_a * fp0_b) + ((fp_b)*fp0_b)/scale
	}

	return Decimal{fp: result}
}

// Div divides f by f0 returning a Decimal. If either operand is NaN, NaN is returned
func (f Decimal) Div(f0 Decimal) Decimal {
	if f.IsNaN() || f0.IsNaN() {
		return NaN
	}
	return NewF(f.Float() / f0.Float())
}

func sign(fp int64) int64 {
	if fp < 0 {
		return -1
	}
	return 1
}

// Round returns a rounded (half-up, away from zero) to n decimal places
func (f Decimal) Round(n int) Decimal {
	if f.IsNaN() {
		return NaN
	}

	fraction := f.fp % scale
	f0 := fraction / int64(math.Pow10(nPlaces-n-1))
	digit := abs(f0 % 10)
	f0 = (f0 / 10)
	if digit >= 5 {
		f0 += 1 * sign(f.fp)
	}
	f0 = f0 * int64(math.Pow10(nPlaces-n))

	intpart := f.fp - fraction
	fp := intpart + f0

	return Decimal{fp: fp}
}

// Equal returns true if the f == f0. If either operand is NaN, false is returned. Use IsNaN() to test for NaN
func (f Decimal) Equal(f0 Decimal) bool {
	if f.IsNaN() || f0.IsNaN() {
		return false
	}
	return f.Cmp(f0) == 0
}

// GreaterThan tests Cmp() for 1
func (f Decimal) GreaterThan(f0 Decimal) bool {
	return f.Cmp(f0) == 1
}

// GreaterThaOrEqual tests Cmp() for 1 or 0
func (f Decimal) GreaterThanOrEqual(f0 Decimal) bool {
	cmp := f.Cmp(f0)
	return cmp == 1 || cmp == 0
}

// LessThan tests Cmp() for -1
func (f Decimal) LessThan(f0 Decimal) bool {
	return f.Cmp(f0) == -1
}

// LessThan tests Cmp() for -1 or 0
func (f Decimal) LessThanOrEqual(f0 Decimal) bool {
	cmp := f.Cmp(f0)
	return cmp == -1 || cmp == 0
}

// Cmp compares two Decimal. If f == f0, return 0. If f > f0, return 1. If f < f0, return -1. If both are NaN, return 0. If f is NaN, return 1. If f0 is NaN, return -1
func (f Decimal) Cmp(f0 Decimal) int {
	if f.IsNaN() && f0.IsNaN() {
		return 0
	}
	if f.IsNaN() {
		return 1
	}
	if f0.IsNaN() {
		return -1
	}

	if f.fp == f0.fp {
		return 0
	}
	if f.fp < f0.fp {
		return -1
	}
	return 1
}

// String converts a Decimal to a string, dropping trailing zeros
func (f Decimal) String() string {
	s, point := f.tostr()
	if point == -1 {
		return s
	}
	index := len(s) - 1
	for ; index != point; index-- {
		if s[index] != '0' {
			return s[:index+1]
		}
	}
	return s[:point]
}

// StringN converts a Decimal to a String with a specified number of decimal places, truncating as required
func (f Decimal) StringN(decimals int) string {

	s, point := f.tostr()

	if point == -1 {
		return s
	}
	if decimals == 0 {
		return s[:point]
	} else {
		return s[:point+decimals+1]
	}
}

func (f Decimal) tostr() (string, int) {
	fp := f.fp
	if fp == 0 {
		return "0." + zeros, 1
	}
	if fp == nan {
		return "NaN", -1
	}

	b := make([]byte, 24)
	b = itoa(b, fp)

	return string(b), len(b) - nPlaces - 1
}

func itoa(buf []byte, val int64) []byte {
	neg := val < 0
	if neg {
		val = val * -1
	}

	i := len(buf) - 1
	idec := i - nPlaces
	for val >= 10 || i >= idec {
		buf[i] = byte(val%10 + '0')
		i--
		if i == idec {
			buf[i] = '.'
			i--
		}
		val /= 10
	}
	buf[i] = byte(val + '0')
	if neg {
		i--
		buf[i] = '-'
	}
	return buf[i:]
}

// Int return the integer portion of the Decimal, or 0 if NaN
func (f Decimal) Int() int64 {
	if f.IsNaN() {
		return 0
	}
	return f.fp / scale
}

// Frac return the fractional portion of the Decimal, or NaN if NaN
func (f Decimal) Frac() float64 {
	if f.IsNaN() {
		return math.NaN()
	}
	return float64(f.fp%scale) / float64(scale)
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface
func (f *Decimal) UnmarshalBinary(data []byte) error {
	fp, n := binary.Varint(data)
	if n < 0 {
		return errFormat
	}
	f.fp = fp
	return nil
}

// MarshalBinary implements the encoding.BinaryMarshaler interface.
func (f Decimal) MarshalBinary() (data []byte, err error) {
	var buffer [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buffer[:], f.fp)
	return buffer[:n], nil
}

// WriteTo write the Decimal to an io.Writer, returning the number of bytes written
func (f Decimal) WriteTo(w io.ByteWriter) error {
	return writeVarint(w, f.fp)
}

// ReadFrom reads a Decimal from an io.Reader
func ReadFrom(r io.ByteReader) (Decimal, error) {
	fp, err := binary.ReadVarint(r)
	if err != nil {
		return NaN, err
	}
	return Decimal{fp: fp}, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (f *Decimal) UnmarshalJSON(bytes []byte) error {
	s := strings.Trim(string(bytes), `"`)
	if s == "null" {
		return nil
	}
	if s == "\"NaN\"" {
		*f = NaN
		return nil
	}

	fixed, err := NewSErr(s)
	*f = fixed
	if err != nil {
		return fmt.Errorf("error decoding string '%s': %s", s, err)
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
func (f Decimal) MarshalJSON() ([]byte, error) {
	if f.IsNaN() {
		return []byte("\"NaN\""), nil
	}
	buffer := make([]byte, 24)
	return itoa(buffer, f.fp), nil
}
