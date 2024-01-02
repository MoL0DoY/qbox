package systems

import (
	"qBox/drivers/skm2/data"
	"qBox/models"
	"qBox/services/convert"
	"sort"
)

type FirstSystem struct {
	System  *models.SystemDevice
	grabber data.Grabber
}

func (firstSystem *FirstSystem) PopulateFromBytes(b []byte) {
	firstSystem.grabber = data.Grabber{Datum: b}
	firstSystem.populateEnergy()

	firstSystem.populateM1()
	firstSystem.populateM2()

	firstSystem.populateV1()
	firstSystem.populateV2()

	firstSystem.populateGM1()
	firstSystem.populateGM2()
	firstSystem.populateGV1()
	firstSystem.populateGV2()

	firstSystem.populateT1()
	firstSystem.populateT2()
	firstSystem.populateT3()

	firstSystem.populateP1()
	firstSystem.populateP2()

	firstSystem.populateTime()
}

func (firstSystem *FirstSystem) populateEnergy() {
	var valueBytes [4]byte
	var grabValue []byte

	type Block struct {
		data   []byte
		factor float32
	}

	blocks := map[int]Block{
		1: {[]byte{0x04, 0x07}, 0.01},
		2: {[]byte{0x04, 0x06}, 0.001},
		3: {[]byte{0x84, 0x80, 0x40, 0x07}, 0.01},
		4: {[]byte{0x84, 0x80, 0x40, 0x06}, 0.001},
	}

	// Требуется гарантированная сортировка, так как Q3 не должен попадать вместо Q1
	// https://blog.golang.org/go-maps-in-action#TOC_7.
	var keys []int
	for k := range blocks {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	for _, key := range keys {
		grabValue = firstSystem.grabber.GrabValueBytes(blocks[key].data, len(valueBytes))
		if cap(valueBytes) == len(grabValue) {
			invertBytes := convert.InvertBytes(grabValue)
			for i := range invertBytes {
				valueBytes[i] = invertBytes[i]
			}
			firstSystem.System.Status = true
			firstSystem.System.SigmaQ = float64(float32(convert.ToLong(valueBytes)) * blocks[key].factor)
			return
		}
	}
}

func (firstSystem *FirstSystem) populateV1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x04, 0x13}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.V1 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
		return
	}
	// На одном из ЦТП произведена замена плат и VID сменился на 0x14 (Очень похоже на протокол SKU-02)
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x04, 0x14}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.V1 = float64(float32(convert.ToLong(valueBytes)) * 0.01)
		return
	}
}

func (firstSystem *FirstSystem) populateV2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x84, 0x40, 0x13}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.V2 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
		return
	}
	// На одном из ЦТП произведена замена плат и VID сменился на 0x14 (Очень похоже на протокол SKU-02)
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x84, 0x40, 0x14}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.V2 = float64(float32(convert.ToLong(valueBytes)) * 0.01)
		return
	}
}

func (firstSystem *FirstSystem) populateM1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x04, 0x1B}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.M1 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
		return
	}
}

func (firstSystem *FirstSystem) populateM2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x84, 0x40, 0x1B}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.M2 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
	}
}

func (firstSystem *FirstSystem) populateGV1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x05, 0x3E}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.GV1 = convert.ToFloat(valueBytes)
	}
}

func (firstSystem *FirstSystem) populateGV2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x85, 0x40, 0x3E}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.GV2 = convert.ToFloat(valueBytes)
	}
}

func (firstSystem *FirstSystem) populateGM1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x05, 0x56}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.GM1 = convert.ToFloat(valueBytes)
	}
}

func (firstSystem *FirstSystem) populateGM2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x85, 0x40, 0x56}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.GM2 = convert.ToFloat(valueBytes)
	}
}

func (firstSystem *FirstSystem) populateT1() {
	var valueBytes [2]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x02, 0x59}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0], valueBytes[1] = grabValue[1], grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.T1 = float32(convert.ToWord(valueBytes)) * 0.01
	}
}

func (firstSystem *FirstSystem) populateT2() {
	var valueBytes [2]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x02, 0x5D}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0], valueBytes[1] = grabValue[1], grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.T2 = float32(convert.ToWord(valueBytes)) * 0.01
	}
}

func (firstSystem *FirstSystem) populateT3() {
	var valueBytes [2]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x02, 0x65}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0], valueBytes[1] = grabValue[1], grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.T3 = float32(convert.ToWord(valueBytes)) * 0.01
	}
}

func (firstSystem *FirstSystem) populateP1() {
	var valueBytes [3]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x03, 0x68}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[2]
		valueBytes[1] = grabValue[1]
		valueBytes[2] = grabValue[0]
		valueToConvert := [4]byte{0x00, grabValue[2], grabValue[1], grabValue[0]}
		firstSystem.System.Status = true
		firstSystem.System.P1 = float32(convert.ToLong(valueToConvert)) * 0.0001
	}
}

func (firstSystem *FirstSystem) populateP2() {
	var valueBytes [3]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x83, 0x40, 0x68}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[2]
		valueBytes[1] = grabValue[1]
		valueBytes[2] = grabValue[0]
		valueToConvert := [4]byte{0x00, grabValue[2], grabValue[1], grabValue[0]}
		firstSystem.System.Status = true
		firstSystem.System.P2 = float32(convert.ToLong(valueToConvert)) * 0.0001
	}
}

func (firstSystem *FirstSystem) populateTime() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = firstSystem.grabber.GrabValueBytes([]byte{0x84, 0x40, 0x24}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		firstSystem.System.Status = true
		firstSystem.System.TimeRunSys = convert.ToLong(valueBytes)
	}
}
