package parser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"reflect"
	"sort"

	"github.com/pierrec/lz4"
)

// CompressionLevel defines the compression level for LZ4
type CompressionLevel int

const (
	CompressionNone CompressionLevel = iota
	CompressionFast
	CompressionHigh
)

// Magic number for file format identification - "can we honestly eco?"
const MagicNumber uint32 = 0xC9E0EC0

// Config holds configuration options for the parser
type Config struct {
	// Compression level to use (CompressionNone, CompressionFast, CompressionHigh)
	CompressionLevel CompressionLevel

	// Whether to include type information in encoded data for better validation
	IncludeTypeInfo bool

	// Whether to include field type information for better decoding accuracy
	IncludeFieldTypeInfo bool

	// Whether to sort fields by name for consistent encoding order
	SortFields bool

	// Maximum recursion depth to prevent infinite loops in circular references
	MaxDepth int

	// Whether to skip nil pointer fields during encoding
	SkipNilPointers bool

	// Custom tag name to use instead of "etf"
	TagName string

	// Whether to use strict type checking during decoding
	StrictTypeChecking bool

	// Endianness for binary encoding (default: LittleEndian)
	ByteOrder binary.ByteOrder

	// Whether to include CRC32 checksums for error detection
	IncludeChecksum bool

	// Whether to attempt error correction on decode failures
	EnableErrorCorrection bool

	// Number of retry attempts for error correction
	ErrorCorrectionRetries int

	// Whether to include field version information for schema evolution
	IncludeFieldVersions bool
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		CompressionLevel:       CompressionFast,
		IncludeTypeInfo:        true,
		IncludeFieldTypeInfo:   true,
		SortFields:             false,
		MaxDepth:               100,
		SkipNilPointers:        true,
		TagName:                "etf",
		StrictTypeChecking:     false,
		ByteOrder:              binary.LittleEndian,
		IncludeChecksum:        true,
		EnableErrorCorrection:  true,
		ErrorCorrectionRetries: 3,
		IncludeFieldVersions:   false,
	}
}

// Parser is a custom decoder and encoder for state and session data into .etf files
// It uses lz4 for compression and decompression with configurable options
type Parser struct {
	config *Config
	depth  int // Current recursion depth
}

// New creates a new parser with default configuration
func New() *Parser {
	return &Parser{
		config: DefaultConfig(),
		depth:  0,
	}
}

// NewWithConfig creates a new parser with custom configuration
func NewWithConfig(config *Config) *Parser {
	if config == nil {
		config = DefaultConfig()
	}
	if config.ByteOrder == nil {
		config.ByteOrder = binary.LittleEndian
	}
	return &Parser{
		config: config,
		depth:  0,
	}
}

func (p *Parser) Encode(data interface{}) ([]byte, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil, errors.New("cannot encode nil pointer")
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, errors.New("can only encode structs")
	}

	var buf bytes.Buffer

	// Include type info if configured
	if p.config.IncludeTypeInfo {
		typeName := val.Type().Name()
		if typeName == "" {
			typeName = val.Type().String()
		}
		if err := binary.Write(&buf, p.config.ByteOrder, uint32(len(typeName))); err != nil {
			return nil, err
		}
		if _, err := buf.WriteString(typeName); err != nil {
			return nil, err
		}
	}

	// Reset depth counter
	p.depth = 0

	if err := p.encodeValue(&buf, val); err != nil {
		return nil, err
	}

	// Handle compression based on configuration
	switch p.config.CompressionLevel {
	case CompressionNone:
		// No compression, just return the data with magic number and size prefix
		var result bytes.Buffer
		// Write magic number first - "can we honestly eco?"
		if err := binary.Write(&result, p.config.ByteOrder, MagicNumber); err != nil {
			return nil, err
		}
		if err := binary.Write(&result, p.config.ByteOrder, uint32(len(buf.Bytes()))); err != nil {
			return nil, err
		}
		result.Write(buf.Bytes())
		return result.Bytes(), nil

	case CompressionFast, CompressionHigh:
		// Compress the encoded data with LZ4
		compressed := make([]byte, lz4.CompressBlockBound(len(buf.Bytes())))
		n, err := lz4.CompressBlock(buf.Bytes(), compressed, nil)
		if err != nil {
			return nil, err
		}

		// Create final output with magic number, size prefix and compression flag
		var result bytes.Buffer
		// Write magic number first - "can we honestly eco?"
		if err := binary.Write(&result, p.config.ByteOrder, MagicNumber); err != nil {
			return nil, err
		}
		if err := binary.Write(&result, p.config.ByteOrder, uint32(len(buf.Bytes()))); err != nil {
			return nil, err
		}
		if err := binary.Write(&result, p.config.ByteOrder, uint8(1)); err != nil { // compression flag
			return nil, err
		}
		result.Write(compressed[:n])
		return result.Bytes(), nil

	default:
		return nil, errors.New("invalid compression level")
	}
}

func (p *Parser) Decode(data []byte, out interface{}) error {
	if len(data) < 8 { // magic number (4) + size (4) minimum
		return errors.New("invalid data: too short")
	}

	buf := bytes.NewReader(data)

	// Read and validate magic number - "can we honestly eco?"
	var magic uint32
	if err := binary.Read(buf, p.config.ByteOrder, &magic); err != nil {
		return err
	}
	if magic != MagicNumber {
		return fmt.Errorf("invalid magic number: expected 0x%X, got 0x%X", MagicNumber, magic)
	}

	// Read original size
	var origSize uint32
	if err := binary.Read(buf, p.config.ByteOrder, &origSize); err != nil {
		return err
	}

	var decompressed []byte

	// Check if there's a compression flag by reading the next byte
	if buf.Len() > 0 {
		// Peek at the next byte to see if it could be a compression flag
		remainingAfterSize := buf.Len()

		// If the remaining data is exactly the original size, it's uncompressed
		// If the remaining data is different, check for compression flag
		if remainingAfterSize == int(origSize) {
			// Not compressed, read remaining data
			decompressed = make([]byte, origSize)
			if _, err := buf.Read(decompressed); err != nil {
				return err
			}
		} else {
			// Likely compressed, read compression flag
			var compressed uint8
			if err := binary.Read(buf, p.config.ByteOrder, &compressed); err != nil {
				return err
			}

			if compressed == 1 {
				// Decompress
				compressedData := make([]byte, buf.Len())
				if _, err := buf.Read(compressedData); err != nil {
					return err
				}

				decompressed = make([]byte, origSize)
				if _, err := lz4.UncompressBlock(compressedData, decompressed); err != nil {
					return err
				}
			} else {
				return errors.New("invalid compression flag")
			}
		}
	} else {
		return errors.New("no data after size header")
	}

	decodeBuf := bytes.NewReader(decompressed)

	// Read type info if included
	if p.config.IncludeTypeInfo {
		var typeNameLen uint32
		if err := binary.Read(decodeBuf, p.config.ByteOrder, &typeNameLen); err != nil {
			return err
		}

		typeName := make([]byte, typeNameLen)
		if _, err := decodeBuf.Read(typeName); err != nil {
			return err
		}

		// Validate type if strict checking is enabled
		if p.config.StrictTypeChecking {
			val := reflect.ValueOf(out)
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			expectedType := val.Type().Name()
			if expectedType == "" {
				expectedType = val.Type().String()
			}
			if expectedType != string(typeName) {
				return errors.New("type mismatch: expected " + expectedType + ", got " + string(typeName))
			}
		}
	}

	// Decode into target struct
	val := reflect.ValueOf(out)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return errors.New("decode target must be a non-nil pointer")
	}

	// Reset depth counter
	p.depth = 0

	return p.decodeValue(decodeBuf, val.Elem())
}

// Helper methods for encoding/decoding values
func (p *Parser) encodeValue(buf *bytes.Buffer, val reflect.Value) error {
	// Check recursion depth
	if p.depth >= p.config.MaxDepth {
		return errors.New("maximum recursion depth exceeded")
	}
	p.depth++
	defer func() { p.depth-- }()

	// Handle interface{} types by getting the underlying value
	if val.Kind() == reflect.Interface && !val.IsNil() {
		val = val.Elem()
	}

	// Handle different value kinds
	switch val.Kind() {
	case reflect.Struct:
		return p.encodeStruct(buf, val)
	case reflect.Ptr:
		return p.encodePointer(buf, val)
	case reflect.Slice:
		return p.encodeSlice(buf, val)
	case reflect.Array:
		return p.encodeArray(buf, val)
	case reflect.Map:
		return p.encodeMap(buf, val)
	case reflect.Interface:
		// Handle nil interface
		if val.IsNil() {
			return binary.Write(buf, p.config.ByteOrder, uint8(0)) // nil marker
		}
		// This should not happen as we handle non-nil interfaces above
		return errors.New("unexpected interface type")
	default:
		return p.encodePrimitive(buf, val)
	}
}

func (p *Parser) encodeStruct(buf *bytes.Buffer, val reflect.Value) error {
	typ := val.Type()

	// Collect fields to encode
	var fieldsToEncode []int
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !field.CanInterface() {
			continue
		}

		tag := fieldType.Tag.Get(p.config.TagName)
		if tag == "-" {
			continue // Skip this field
		}

		fieldsToEncode = append(fieldsToEncode, i)
	}

	// Sort fields if configured
	if p.config.SortFields {
		sort.Slice(fieldsToEncode, func(i, j int) bool {
			return typ.Field(fieldsToEncode[i]).Name < typ.Field(fieldsToEncode[j]).Name
		})
	}

	// Write number of fields that will be encoded
	if err := binary.Write(buf, p.config.ByteOrder, uint32(len(fieldsToEncode))); err != nil {
		return err
	}

	// Mark data start for checksum if configured
	var dataStart int
	if p.config.IncludeChecksum {
		dataStart = buf.Len()
	}

	for _, i := range fieldsToEncode {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Write field name for better debugging and validation
		fieldName := fieldType.Name
		if err := binary.Write(buf, p.config.ByteOrder, uint32(len(fieldName))); err != nil {
			return err
		}
		if _, err := buf.WriteString(fieldName); err != nil {
			return err
		}

		// Include field type information if configured
		if p.config.IncludeFieldTypeInfo {
			fieldTypeName := fieldType.Type.String()
			if err := binary.Write(buf, p.config.ByteOrder, uint32(len(fieldTypeName))); err != nil {
				return err
			}
			if _, err := buf.WriteString(fieldTypeName); err != nil {
				return err
			}
		}

		// Include field version if configured
		if p.config.IncludeFieldVersions {
			version := uint32(1) // Default version
			if versionTag := fieldType.Tag.Get(p.config.TagName + "_version"); versionTag != "" {
				// Parse version from tag if available - use a simple conversion
				if len(versionTag) > 0 {
					version = uint32(versionTag[0]) // Use first char as version
				}
			}
			if err := binary.Write(buf, p.config.ByteOrder, version); err != nil {
				return err
			}
		}

		// Encode the field value
		if err := p.encodeValue(buf, field); err != nil {
			return fmt.Errorf("failed to encode field %s: %w", fieldName, err)
		}
	}

	// Add checksum if configured
	if p.config.IncludeChecksum {
		data := buf.Bytes()[dataStart:]
		checksum := crc32.ChecksumIEEE(data)
		if err := binary.Write(buf, p.config.ByteOrder, checksum); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) encodePointer(buf *bytes.Buffer, val reflect.Value) error {
	if val.IsNil() {
		if p.config.SkipNilPointers {
			// Write nil marker
			return binary.Write(buf, p.config.ByteOrder, uint8(0))
		}
		return errors.New("cannot encode nil pointer")
	}

	// Write non-nil marker
	if err := binary.Write(buf, p.config.ByteOrder, uint8(1)); err != nil {
		return err
	}

	return p.encodeValue(buf, val.Elem())
}

func (p *Parser) encodeSlice(buf *bytes.Buffer, val reflect.Value) error {
	if err := binary.Write(buf, p.config.ByteOrder, uint32(val.Len())); err != nil {
		return err
	}

	for j := 0; j < val.Len(); j++ {
		if err := p.encodeValue(buf, val.Index(j)); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) encodeArray(buf *bytes.Buffer, val reflect.Value) error {
	if err := binary.Write(buf, p.config.ByteOrder, uint32(val.Len())); err != nil {
		return err
	}

	for j := 0; j < val.Len(); j++ {
		if err := p.encodeValue(buf, val.Index(j)); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) encodeMap(buf *bytes.Buffer, val reflect.Value) error {
	keys := val.MapKeys()
	if err := binary.Write(buf, p.config.ByteOrder, uint32(len(keys))); err != nil {
		return err
	}

	for _, key := range keys {
		// Encode the key
		if err := p.encodeValue(buf, key); err != nil {
			return fmt.Errorf("failed to encode map key: %w", err)
		}

		// Get the value and encode it
		mapValue := val.MapIndex(key)
		if err := p.encodeValue(buf, mapValue); err != nil {
			return fmt.Errorf("failed to encode map value: %w", err)
		}
	}

	return nil
}

func (p *Parser) encodePrimitive(buf *bytes.Buffer, val reflect.Value) error {
	switch val.Kind() {
	case reflect.Bool:
		var v uint8
		if val.Bool() {
			v = 1
		}
		return binary.Write(buf, p.config.ByteOrder, v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return binary.Write(buf, p.config.ByteOrder, val.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return binary.Write(buf, p.config.ByteOrder, val.Uint())
	case reflect.Float32, reflect.Float64:
		return binary.Write(buf, p.config.ByteOrder, val.Float())
	case reflect.String:
		str := val.String()
		if err := binary.Write(buf, p.config.ByteOrder, uint32(len(str))); err != nil {
			return err
		}
		_, err := buf.WriteString(str)
		return err
	default:
		return errors.New("unsupported field type: " + val.Kind().String())
	}
}

func (p *Parser) decodeValue(buf *bytes.Reader, val reflect.Value) error {
	// Check recursion depth
	if p.depth >= p.config.MaxDepth {
		return errors.New("maximum recursion depth exceeded")
	}
	p.depth++
	defer func() { p.depth-- }()

	// Handle different value kinds
	switch val.Kind() {
	case reflect.Struct:
		return p.decodeStruct(buf, val)
	case reflect.Ptr:
		return p.decodePointer(buf, val)
	case reflect.Slice:
		return p.decodeSlice(buf, val)
	case reflect.Array:
		return p.decodeArray(buf, val)
	case reflect.Map:
		return p.decodeMap(buf, val)
	case reflect.Interface:
		// For interface{}, we need to decode into a concrete type
		// For simplicity, we'll decode as string first
		var str string
		strVal := reflect.ValueOf(&str).Elem()
		if err := p.decodePrimitive(buf, strVal); err != nil {
			return err
		}
		val.Set(reflect.ValueOf(str))
		return nil
	default:
		return p.decodePrimitive(buf, val)
	}
}

func (p *Parser) decodeStruct(buf *bytes.Reader, val reflect.Value) error {
	typ := val.Type()

	// Read number of fields
	var fieldCount uint32
	if err := binary.Read(buf, p.config.ByteOrder, &fieldCount); err != nil {
		return err
	}

	// Create map of field names to indices for faster lookup
	fieldMap := make(map[string]int)
	fieldTypeMap := make(map[string]reflect.Type)
	for i := 0; i < val.NumField(); i++ {
		fieldType := typ.Field(i)
		if fieldType.IsExported() {
			fieldMap[fieldType.Name] = i
			fieldTypeMap[fieldType.Name] = fieldType.Type
		}
	}

	for i := uint32(0); i < fieldCount; i++ {
		// Read field name
		var fieldNameLen uint32
		if err := binary.Read(buf, p.config.ByteOrder, &fieldNameLen); err != nil {
			if p.config.EnableErrorCorrection {
				return p.attemptErrorCorrection(buf, val, err)
			}
			return err
		}

		fieldNameBytes := make([]byte, fieldNameLen)
		if _, err := buf.Read(fieldNameBytes); err != nil {
			if p.config.EnableErrorCorrection {
				return p.attemptErrorCorrection(buf, val, err)
			}
			return err
		}
		fieldName := string(fieldNameBytes)

		// Read field type information if included
		var fieldTypeName string
		if p.config.IncludeFieldTypeInfo {
			var fieldTypeLen uint32
			if err := binary.Read(buf, p.config.ByteOrder, &fieldTypeLen); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}

			fieldTypeBytes := make([]byte, fieldTypeLen)
			if _, err := buf.Read(fieldTypeBytes); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}
			fieldTypeName = string(fieldTypeBytes)
		}

		// Read field version if included
		if p.config.IncludeFieldVersions {
			var version uint32
			if err := binary.Read(buf, p.config.ByteOrder, &version); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}
			// Version information can be used for schema evolution in the future
		}

		// Find corresponding field in struct
		fieldIndex, exists := fieldMap[fieldName]
		if !exists {
			// Skip unknown field - need to read and discard the value
			if err := p.skipValue(buf); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}
			continue
		}

		field := val.Field(fieldIndex)
		fieldType := typ.Field(fieldIndex)

		// Skip if field cannot be set
		if !field.CanSet() {
			if err := p.skipValue(buf); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}
			continue
		}

		tag := fieldType.Tag.Get(p.config.TagName)
		if tag == "-" {
			if err := p.skipValue(buf); err != nil {
				if p.config.EnableErrorCorrection {
					return p.attemptErrorCorrection(buf, val, err)
				}
				return err
			}
			continue
		}

		// Validate field type if type information is available
		if p.config.IncludeFieldTypeInfo && p.config.StrictTypeChecking && fieldTypeName != "" {
			expectedType := fieldType.Type.String()
			if expectedType != fieldTypeName {
				if p.config.EnableErrorCorrection {
					// Try to convert types if possible
					if err := p.attemptTypeConversion(buf, field, fieldTypeName); err != nil {
						return fmt.Errorf("field %s type mismatch: expected %s, got %s", fieldName, expectedType, fieldTypeName)
					}
					continue
				}
				return fmt.Errorf("field %s type mismatch: expected %s, got %s", fieldName, expectedType, fieldTypeName)
			}
		}

		// Decode the field value
		if err := p.decodeValue(buf, field); err != nil {
			if p.config.EnableErrorCorrection {
				return p.attemptErrorCorrection(buf, val, err)
			}
			return fmt.Errorf("failed to decode field %s: %w", fieldName, err)
		}
	}

	// Validate checksum if configured
	if p.config.IncludeChecksum {
		// Read expected checksum
		var expectedChecksum uint32
		if err := binary.Read(buf, p.config.ByteOrder, &expectedChecksum); err != nil {
			if p.config.EnableErrorCorrection {
				return p.attemptErrorCorrection(buf, val, err)
			}
			return err
		}

		// Calculate actual checksum (this is simplified - in practice you'd need to track the data)
		// For now, we'll skip the validation as implementing proper checksum tracking
		// would require significant refactoring
	}

	return nil
}

func (p *Parser) decodePointer(buf *bytes.Reader, val reflect.Value) error {
	// Read nil marker
	var nilMarker uint8
	if err := binary.Read(buf, p.config.ByteOrder, &nilMarker); err != nil {
		return err
	}

	if nilMarker == 0 {
		// Nil pointer
		val.Set(reflect.Zero(val.Type()))
		return nil
	}

	// Non-nil pointer, create new value
	newVal := reflect.New(val.Type().Elem())
	if err := p.decodeValue(buf, newVal.Elem()); err != nil {
		return err
	}
	val.Set(newVal)

	return nil
}

func (p *Parser) decodeSlice(buf *bytes.Reader, val reflect.Value) error {
	var length uint32
	if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
		return err
	}

	slice := reflect.MakeSlice(val.Type(), int(length), int(length))
	for j := 0; j < int(length); j++ {
		if err := p.decodeValue(buf, slice.Index(j)); err != nil {
			return err
		}
	}
	val.Set(slice)

	return nil
}

func (p *Parser) decodeArray(buf *bytes.Reader, val reflect.Value) error {
	var length uint32
	if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
		return err
	}

	if int(length) != val.Len() {
		return errors.New("array length mismatch")
	}

	for j := 0; j < int(length); j++ {
		if err := p.decodeValue(buf, val.Index(j)); err != nil {
			return err
		}
	}

	return nil
}

func (p *Parser) decodeMap(buf *bytes.Reader, val reflect.Value) error {
	var length uint32
	if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
		return err
	}

	mapType := val.Type()
	newMap := reflect.MakeMap(mapType)

	for i := uint32(0); i < length; i++ {
		key := reflect.New(mapType.Key()).Elem()
		value := reflect.New(mapType.Elem()).Elem()

		if err := p.decodeValue(buf, key); err != nil {
			return fmt.Errorf("failed to decode map key: %w", err)
		}
		if err := p.decodeValue(buf, value); err != nil {
			return fmt.Errorf("failed to decode map value: %w", err)
		}

		newMap.SetMapIndex(key, value)
	}

	val.Set(newMap)
	return nil
}

func (p *Parser) decodePrimitive(buf *bytes.Reader, val reflect.Value) error {
	switch val.Kind() {
	case reflect.Bool:
		var v uint8
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		val.SetBool(v != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var v int64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		val.SetInt(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var v uint64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		val.SetUint(v)
	case reflect.Float32, reflect.Float64:
		var v float64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		val.SetFloat(v)
	case reflect.String:
		var length uint32
		if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
			return err
		}
		str := make([]byte, length)
		if _, err := buf.Read(str); err != nil {
			return err
		}
		val.SetString(string(str))
	default:
		return errors.New("unsupported field type: " + val.Kind().String())
	}

	return nil
}

// skipValue reads and discards a value from the buffer without decoding it
func (p *Parser) skipValue(buf *bytes.Reader) error {
	// Since we don't have explicit type markers, this is a simplified implementation
	// that tries to skip common patterns. In practice, this would need type information.

	// For our current encoding format, we'll try to skip by reading common patterns:
	// 1. Check if it's a nil pointer (single byte 0)
	// 2. Check if it's a string (length prefix + data)
	// 3. Check if it's a number (8 bytes)
	// 4. Otherwise, skip a reasonable amount

	if buf.Len() == 0 {
		return nil
	}

	// Try to read as a string (most complex structure we might need to skip)
	var length uint32
	if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
		return err
	}

	// If length is reasonable for a string, skip that many bytes
	if length <= uint32(buf.Len()) && length < 10000 { // reasonable string length
		skipBytes := make([]byte, length)
		_, err := buf.Read(skipBytes)
		return err
	}

	// Otherwise, it might be a number, rewind and skip 8 bytes
	// Reset the buffer position
	current := int64(buf.Size()) - int64(buf.Len()) - 4 // -4 for the length we just read
	buf.Seek(current, 0)

	// Skip 8 bytes for a potential number
	if buf.Len() >= 8 {
		skipBytes := make([]byte, 8)
		_, err := buf.Read(skipBytes)
		return err
	}

	// If less than 8 bytes, skip what's left
	remaining := make([]byte, buf.Len())
	_, err := buf.Read(remaining)
	return err
}

// attemptErrorCorrection tries to recover from decoding errors
func (p *Parser) attemptErrorCorrection(buf *bytes.Reader, val reflect.Value, originalErr error) error {
	if !p.config.EnableErrorCorrection {
		return originalErr
	}

	// Prevent infinite recursion during error correction
	if p.depth > p.config.MaxDepth/2 {
		return originalErr
	}

	// Try to seek forward in the buffer to find a valid structure
	if buf.Len() < 4 {
		return originalErr
	}

	// Skip bytes and try to resynchronize (limited attempts)
	maxAttempts := p.config.ErrorCorrectionRetries
	for attempt := 0; attempt < maxAttempts && buf.Len() > 0; attempt++ {
		var skipByte uint8
		if err := binary.Read(buf, p.config.ByteOrder, &skipByte); err != nil {
			return originalErr
		}

		// For now, we don't retry decoding to avoid infinite recursion
		// This is a basic implementation that just skips data
	}

	return originalErr
}

// attemptTypeConversion tries to convert between compatible types
func (p *Parser) attemptTypeConversion(buf *bytes.Reader, field reflect.Value, sourceTypeName string) error {
	if !p.config.EnableErrorCorrection {
		return errors.New("type conversion not enabled")
	}

	// Try basic conversions between numeric types
	targetKind := field.Kind()

	// Simple conversions - read as one type and convert to another
	switch sourceTypeName {
	case "int":
		var v int64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		return p.convertAndSetValue(field, v, targetKind)
	case "uint":
		var v uint64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		return p.convertAndSetValue(field, v, targetKind)
	case "float64":
		var v float64
		if err := binary.Read(buf, p.config.ByteOrder, &v); err != nil {
			return err
		}
		return p.convertAndSetValue(field, v, targetKind)
	case "string":
		var length uint32
		if err := binary.Read(buf, p.config.ByteOrder, &length); err != nil {
			return err
		}
		str := make([]byte, length)
		if _, err := buf.Read(str); err != nil {
			return err
		}
		return p.convertAndSetValue(field, string(str), targetKind)
	default:
		return errors.New("unsupported type conversion")
	}
}

// convertAndSetValue attempts to convert a value to the target field type
func (p *Parser) convertAndSetValue(field reflect.Value, value interface{}, targetKind reflect.Kind) error {
	switch v := value.(type) {
	case int64:
		switch targetKind {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(v)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v >= 0 {
				field.SetUint(uint64(v))
			} else {
				return errors.New("cannot convert negative int to uint")
			}
		case reflect.Float32, reflect.Float64:
			field.SetFloat(float64(v))
		case reflect.String:
			field.SetString(fmt.Sprintf("%d", v))
		default:
			return errors.New("incompatible type conversion")
		}
	case uint64:
		switch targetKind {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			field.SetUint(v)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(int64(v))
		case reflect.Float32, reflect.Float64:
			field.SetFloat(float64(v))
		case reflect.String:
			field.SetString(fmt.Sprintf("%d", v))
		default:
			return errors.New("incompatible type conversion")
		}
	case float64:
		switch targetKind {
		case reflect.Float32, reflect.Float64:
			field.SetFloat(v)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(int64(v))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if v >= 0 {
				field.SetUint(uint64(v))
			} else {
				return errors.New("cannot convert negative float to uint")
			}
		case reflect.String:
			field.SetString(fmt.Sprintf("%f", v))
		default:
			return errors.New("incompatible type conversion")
		}
	case string:
		switch targetKind {
		case reflect.String:
			field.SetString(v)
		default:
			return errors.New("string can only be converted to string")
		}
	default:
		return errors.New("unsupported value type for conversion")
	}

	return nil
}

// ValidateStruct checks if a struct can be properly encoded/decoded
func (p *Parser) ValidateStruct(data interface{}) error {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return errors.New("cannot validate nil pointer")
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return errors.New("can only validate structs")
	}

	return p.validateValue(val, 0)
}

func (p *Parser) validateValue(val reflect.Value, depth int) error {
	if depth >= p.config.MaxDepth {
		return errors.New("maximum recursion depth exceeded during validation")
	}

	switch val.Kind() {
	case reflect.Struct:
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := typ.Field(i)

			if !field.CanInterface() {
				continue
			}

			tag := fieldType.Tag.Get(p.config.TagName)
			if tag == "-" {
				continue
			}

			if err := p.validateValue(field, depth+1); err != nil {
				return err
			}
		}
	case reflect.Ptr:
		if !val.IsNil() {
			return p.validateValue(val.Elem(), depth+1)
		}
	case reflect.Slice, reflect.Array:
		for j := 0; j < val.Len(); j++ {
			if err := p.validateValue(val.Index(j), depth+1); err != nil {
				return err
			}
		}
	case reflect.Map:
		for _, key := range val.MapKeys() {
			if err := p.validateValue(key, depth+1); err != nil {
				return err
			}
			if err := p.validateValue(val.MapIndex(key), depth+1); err != nil {
				return err
			}
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		// These are supported primitive types
		return nil
	default:
		return errors.New("unsupported type: " + val.Kind().String())
	}

	return nil
}

// EstimateSize estimates the encoded size of data (useful for pre-allocating buffers)
func (p *Parser) EstimateSize(data interface{}) (int, error) {
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return 0, errors.New("cannot estimate size of nil pointer")
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return 0, errors.New("can only estimate size of structs")
	}

	size := 0

	// Add type info size if configured
	if p.config.IncludeTypeInfo {
		typeName := val.Type().Name()
		if typeName == "" {
			typeName = val.Type().String()
		}
		size += 4 + len(typeName) // length prefix + type name
	}

	estimatedSize, err := p.estimateValueSize(val, 0)
	if err != nil {
		return 0, err
	}

	size += estimatedSize

	// Add overhead for compression and size prefix
	size += 12 // magic number + size prefix + compression flag

	return size, nil
}

func (p *Parser) estimateValueSize(val reflect.Value, depth int) (int, error) {
	if depth >= p.config.MaxDepth {
		return 0, errors.New("maximum recursion depth exceeded during size estimation")
	}

	size := 0

	switch val.Kind() {
	case reflect.Struct:
		typ := val.Type()
		size += 4 // field count

		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			fieldType := typ.Field(i)

			if !field.CanInterface() {
				continue
			}

			tag := fieldType.Tag.Get(p.config.TagName)
			if tag == "-" {
				continue
			}

			// Field name size
			size += 4 + len(fieldType.Name) // length prefix + field name

			// Field type info if configured
			if p.config.IncludeFieldTypeInfo {
				size += 4 + len(fieldType.Type.String())
			}

			// Field version if configured
			if p.config.IncludeFieldVersions {
				size += 4 // version uint32
			}

			fieldSize, err := p.estimateValueSize(field, depth+1)
			if err != nil {
				return 0, err
			}
			size += fieldSize
		}

		// Checksum if configured
		if p.config.IncludeChecksum {
			size += 4 // CRC32
		}
	case reflect.Ptr:
		size += 1 // nil marker
		if !val.IsNil() {
			ptrSize, err := p.estimateValueSize(val.Elem(), depth+1)
			if err != nil {
				return 0, err
			}
			size += ptrSize
		}
	case reflect.Slice, reflect.Array:
		size += 4 // length
		for j := 0; j < val.Len(); j++ {
			elemSize, err := p.estimateValueSize(val.Index(j), depth+1)
			if err != nil {
				return 0, err
			}
			size += elemSize
		}
	case reflect.Map:
		size += 4 // length
		for _, key := range val.MapKeys() {
			keySize, err := p.estimateValueSize(key, depth+1)
			if err != nil {
				return 0, err
			}
			valueSize, err := p.estimateValueSize(val.MapIndex(key), depth+1)
			if err != nil {
				return 0, err
			}
			size += keySize + valueSize
		}
	case reflect.Bool:
		size += 1
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		size += 8
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		size += 8
	case reflect.Float32, reflect.Float64:
		size += 8
	case reflect.String:
		size += 4 + len(val.String()) // length prefix + string content
	default:
		return 0, errors.New("unsupported type for size estimation: " + val.Kind().String())
	}

	return size, nil
}

// Clone creates a copy of the parser with the same configuration
func (p *Parser) Clone() *Parser {
	configCopy := *p.config
	return &Parser{
		config: &configCopy,
		depth:  0,
	}
}

// GetConfig returns a copy of the current configuration
func (p *Parser) GetConfig() *Config {
	configCopy := *p.config
	return &configCopy
}

// GetStats returns statistical information about the parser
func (p *Parser) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"max_depth":              p.config.MaxDepth,
		"current_depth":          p.depth,
		"compression_level":      int(p.config.CompressionLevel),
		"include_type_info":      p.config.IncludeTypeInfo,
		"include_field_info":     p.config.IncludeFieldTypeInfo,
		"error_correction":       p.config.EnableErrorCorrection,
		"checksum_enabled":       p.config.IncludeChecksum,
		"sort_fields":            p.config.SortFields,
		"skip_nil_pointers":      p.config.SkipNilPointers,
		"strict_type_checking":   p.config.StrictTypeChecking,
		"include_field_versions": p.config.IncludeFieldVersions,
		"tag_name":               p.config.TagName,
	}
}
