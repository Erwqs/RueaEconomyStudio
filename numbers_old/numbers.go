package numbers

import (
	"encoding/json"
	"fmt"
)

// 128 bit fixed-point number representation
type FixedPoint128 struct {
	Whole    int64
	Fraction int64
}

func NewFixedPoint(whole, fraction int64) FixedPoint128 {
	return FixedPoint128{
		Whole:    whole,
		Fraction: fraction,
	}
}

func NewFixedPointFromString(value string) (FixedPoint128, error) {
	var whole, fraction int64
	n, err := fmt.Sscanf(value, "%d.%06d", &whole, &fraction)
	if n != 2 || err != nil {
		return FixedPoint128{}, fmt.Errorf("invalid FixedPoint format: %s", value)
	}

	return FixedPoint128{
		Whole:    whole,
		Fraction: fraction,
	}, nil
}

func NewFixedPointFromInt[T int64 | int32 | int16 | int8 | int](value T) FixedPoint128 {
	return FixedPoint128{
		Whole:    int64(value),
		Fraction: 0,
	}
}

func NewFixedPointFromFloat(value float64) FixedPoint128 {
	whole := int64(value)
	fraction := int64((value - float64(whole)) * 1e6) // Assuming 6 decimal places
	return FixedPoint128{
		Whole:    whole,
		Fraction: fraction,
	}
}

func (fp FixedPoint128) Add(other FixedPoint128) FixedPoint128 {
	return FixedPoint128{
		Whole:    fp.Whole + other.Whole,
		Fraction: fp.Fraction + other.Fraction,
	}
}

func (fp FixedPoint128) Subtract(other FixedPoint128) FixedPoint128 {
	return FixedPoint128{
		Whole:    fp.Whole - other.Whole,
		Fraction: fp.Fraction - other.Fraction,
	}
}

func (fp FixedPoint128) Multiply(other FixedPoint128) FixedPoint128 {
	whole := fp.Whole*other.Whole + (fp.Fraction*other.Whole+fp.Whole*other.Fraction)/1e6
	fraction := (fp.Fraction * other.Fraction) / 1e6
	return FixedPoint128{
		Whole:    whole,
		Fraction: fraction,
	}
}

func (fp FixedPoint128) Divide(other FixedPoint128) FixedPoint128 {
	if other.Whole == 0 && other.Fraction == 0 {
		panic("division by zero")
	}

	// Convert to float for division
	value := float64(fp.Whole) + float64(fp.Fraction)/1e6
	otherValue := float64(other.Whole) + float64(other.Fraction)/1e6

	result := value / otherValue
	return NewFixedPointFromFloat(result)
}

// Ceil
func (fp FixedPoint128) Ceil() FixedPoint128 {
	if fp.Fraction > 0 {
		return FixedPoint128{
			Whole:    fp.Whole + 1,
			Fraction: 0,
		}
	}
	return fp
}

// Floor
func (fp FixedPoint128) Floor() FixedPoint128 {
	if fp.Fraction < 0 {
		return FixedPoint128{
			Whole:    fp.Whole - 1,
			Fraction: 0,
		}
	}
	return fp
}

// Round
func (fp FixedPoint128) Round() FixedPoint128 {
	if fp.Fraction >= 500000 {
		return FixedPoint128{
			Whole:    fp.Whole + 1,
			Fraction: 0,
		}
	}
	return FixedPoint128{
		Whole:    fp.Whole,
		Fraction: 0,
	}
}

func (fp FixedPoint128) GreaterThan(other FixedPoint128) bool {
	if fp.Whole > other.Whole {
		return true
	} else if fp.Whole < other.Whole {
		return false
	}
	return fp.Fraction > other.Fraction
}

func (fp FixedPoint128) LessThan(other FixedPoint128) bool {
	if fp.Whole < other.Whole {
		return true
	} else if fp.Whole > other.Whole {
		return false
	}
	return fp.Fraction < other.Fraction
}

func (fp FixedPoint128) GreaterThanOrEqual(other FixedPoint128) bool {
	if fp.Whole > other.Whole {
		return true
	} else if fp.Whole < other.Whole {
		return false
	}
	return fp.Fraction >= other.Fraction
}

func (fp FixedPoint128) LessThanOrEqual(other FixedPoint128) bool {
	if fp.Whole < other.Whole {
		return true
	} else if fp.Whole > other.Whole {
		return false
	}
	return fp.Fraction <= other.Fraction
}


func (fp FixedPoint128) String() string {
	return fmt.Sprintf("%d.%06d", fp.Whole, fp.Fraction)
}

func (fp FixedPoint128) Equals(other FixedPoint128) bool {
	return fp.Whole == other.Whole && fp.Fraction == other.Fraction
}

// JSON serialization for FixedPoint
func (fp FixedPoint128) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, `"%s"`, fp.String()), nil
}

// JSON deserialization for FixedPoint
func (fp *FixedPoint128) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	var whole, fraction int64
	n, err := fmt.Sscanf(str, "%d.%06d", &whole, &fraction)
	if n != 2 || err != nil {
		return fmt.Errorf("invalid FixedPoint format: %s", str)
	}

	fp.Whole = whole
	fp.Fraction = fraction
	return nil
}

// Make sure it doesn't cause floating point precision issues
func (fp FixedPoint128) ToFloat() float64 {
	return float64(fp.Whole) + float64(fp.Fraction)/1e6
}

func (fp FixedPoint128) Integer64() int64 {
	return fp.Whole + fp.Fraction/1e6
}

func (fp FixedPoint128) Integer() int {
	return int(fp.Whole) + int(fp.Fraction)/1e6
}

func (fp FixedPoint128) Unsigned64() uint64 {
	return uint64(fp.Whole) + uint64(fp.Fraction)/1e6
}

func (fp FixedPoint128) Unsigned() uint {
	return uint(fp.Whole) + uint(fp.Fraction)/1e6
}

func (fp FixedPoint128) Float64() float64 {
	return float64(fp.Whole) + float64(fp.Fraction)/1e6
}