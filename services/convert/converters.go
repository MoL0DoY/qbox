package convert

import (
	"bytes"
	"encoding/binary"
	"math"
)

// Переворачивает байты и возвращает результат
// Если входные 01 02 03 будет возвращено 03 02 01
func Invert(b []byte) []byte {
	for i := 0; i < len(b)/2; i++ {
		b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
	}
	return b
}

func ToLong(bytes [4]byte) uint32 {
	var amount uint32 = 0
	for i := 0; i <= 3; i++ {
		amount += uint32(uint32(bytes[i]) << uint32(8*(3-i)))
	}
	return amount
}

func ToUint64(bytes []byte) uint64 {
	return binary.BigEndian.Uint64(bytes)
}

func ToFloat(bytes [4]byte) float32 {
	return math.Float32frombits(ToLong(bytes))
}

func ToDouble(bytes []byte) float64 {
	return math.Float64frombits(ToUint64(bytes))
}

func ToWord(bytes [2]byte) uint16 {
	return uint16(uint16(bytes[0])<<8 + uint16(bytes[1]))
}

/*
Инверсное значение байта. Например: 01 => !01 = FE
*/
func ToNotByte(anByte byte) byte {
	return (^anByte) & 0xFF
}

func decodeBcd(bcd []byte) int {
	var x = 0
	for i, b := range bcd {
		hi, lo := int(b>>4), int(b&0x0f)
		if lo == 0x0f && i == len(bcd)-1 {
			return 10*x + hi
		}
		if hi > 9 || lo > 9 {
			return 0
		}
		x = 100*x + 10*hi + lo
	}
	return x
}

// Приведение целого к байтам
// Порядок байт - BigEndian
// Пример: 8704 (dec) будет приведено к 2200 (hex)
func IntToBigEndianBytes(i uint16) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, i)
	return buf.Bytes()
}

// Приведение целого к байтам
// Порядок байт - LittleEndian
// Пример: 8704 (dec) будет приведено к 0022 (hex)
func intToLittleEndian(i uint16) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, i)
	return buf.Bytes()
}

func calculateFloatByPointer(datum []byte, pointer uint8) float32 {
	return ToFloat([4]byte{datum[pointer], datum[pointer+1], datum[pointer+2], datum[pointer+3]})
}

func CalculateLongByPointer(datum []byte, pointer uint8) uint32 {
	return ToLong([4]byte{datum[pointer], datum[pointer+1], datum[pointer+2], datum[pointer+3]})
}

func FloatLittleEndianByPointer(datum []byte, pointer uint8) float32 {
	return ToFloat([4]byte{datum[pointer+3], datum[pointer+2], datum[pointer+1], datum[pointer]})
}

func LongLittleEndianByPointer(datum []byte, pointer uint8) uint32 {
	return ToLong([4]byte{datum[pointer+3], datum[pointer+2], datum[pointer+1], datum[pointer]})
}

func ByteFromBDC(xb byte) (rb byte) { //эта функция преобразует число вида 0х22 в 0d22 (в простонародье - двоично-десятичная коррекция)
	return ((xb&0xF0)>>4)*10 + (xb & 0x0F)
}

func InvertBytes(b []byte) []byte {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

func LongLongLittleEndianByPointer(datum []byte, pointer uint32) uint64 {
	return binary.BigEndian.Uint64([]byte{datum[pointer+7], datum[pointer+6], datum[pointer+5], datum[pointer+4], datum[pointer+3], datum[pointer+2], datum[pointer+1], datum[pointer]})
}

func LongWordLittleEndianByPointer(datum []byte, pointer uint32) uint32 {
	return binary.BigEndian.Uint32([]byte{datum[pointer+3], datum[pointer+2], datum[pointer+1], datum[pointer]})
}
