package systems

import (
	"qBox/drivers/skm2/data"
	"qBox/models"
	"qBox/services/convert"
)

type SecondSystem struct {
	System  *models.SystemDevice
	grabber data.Grabber
}

func (secondSystem *SecondSystem) PopulateFromBytes(b []byte) {
	secondSystem.grabber = data.Grabber{Datum: b}
	secondSystem.populateEnergy()

	secondSystem.populateM1()
	secondSystem.populateM2()

	secondSystem.populateV1()
	secondSystem.populateV2()

	secondSystem.populateGM1()
	secondSystem.populateGM2()
	secondSystem.populateGV1()
	secondSystem.populateGV2()

	secondSystem.populateT1()
	secondSystem.populateT2()

	secondSystem.populateP1()
	secondSystem.populateP2()

	secondSystem.populateTime()
}

func (secondSystem *SecondSystem) populateEnergy() {
	var valueBytes [4]byte
	var grabValue []byte

	type Block struct {
		data   []byte
		factor float32
	}

	blocks := map[int]Block{
		0: {[]byte{0x84, 0x40, 0x07}, 0.01},
		1: {[]byte{0x84, 0x40, 0x06}, 0.001},
	}

	for _, block := range blocks {
		grabValue = secondSystem.grabber.GrabValueBytes(block.data, len(valueBytes))
		if cap(valueBytes) == len(grabValue) {
			valueBytes[0] = grabValue[3]
			valueBytes[1] = grabValue[2]
			valueBytes[2] = grabValue[1]
			valueBytes[3] = grabValue[0]
			secondSystem.System.Status = true
			secondSystem.System.SigmaQ = float64(float32(convert.ToLong(valueBytes)) * block.factor)
			return
		}
	}
}

func (secondSystem *SecondSystem) populateV1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x84, 0xC0, 0x40, 0x13}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.V1 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
	}
}

func (secondSystem *SecondSystem) populateV2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x84, 0x80, 0x80, 0x40, 0x13}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.V2 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
	}
}

func (secondSystem *SecondSystem) populateM1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x84, 0xC0, 0x40, 0x1B}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.M1 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
	}
}

func (secondSystem *SecondSystem) populateM2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x84, 0x80, 0x80, 0x40, 0x1B}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.M2 = float64(float32(convert.ToLong(valueBytes)) * 0.001)
	}
}

func (secondSystem *SecondSystem) populateGV1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x85, 0x80, 0x40, 0x3E}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.GV1 = convert.ToFloat(valueBytes)
	}
}

func (secondSystem *SecondSystem) populateGV2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x85, 0xC0, 0x40, 0x3E}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.GV2 = convert.ToFloat(valueBytes)
	}
}

func (secondSystem *SecondSystem) populateGM1() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x85, 0x80, 0x40, 0x56}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.GM1 = convert.ToFloat(valueBytes)
	}
}

func (secondSystem *SecondSystem) populateGM2() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x85, 0xC0, 0x40, 0x56}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.GM2 = convert.ToFloat(valueBytes)
	}
}

func (secondSystem *SecondSystem) populateT1() {
	var valueBytes [2]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x82, 0x40, 0x59}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0], valueBytes[1] = grabValue[1], grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.T1 = float32(convert.ToWord(valueBytes)) * 0.01
	}
}

func (secondSystem *SecondSystem) populateT2() {
	var valueBytes [2]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x82, 0x40, 0x5D}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0], valueBytes[1] = grabValue[1], grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.T2 = float32(convert.ToWord(valueBytes)) * 0.01
	}
}

func (secondSystem *SecondSystem) populateP1() {
	var valueBytes [3]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x83, 0x80, 0x40, 0x68}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[2]
		valueBytes[1] = grabValue[1]
		valueBytes[2] = grabValue[0]
		valueToConvert := [4]byte{0x00, grabValue[2], grabValue[1], grabValue[0]}
		secondSystem.System.Status = true
		secondSystem.System.P1 = float32(convert.ToLong(valueToConvert)) * 0.0001
	}
}

func (secondSystem *SecondSystem) populateP2() {
	var valueBytes [3]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x83, 0xC0, 0x40, 0x68}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[2]
		valueBytes[1] = grabValue[1]
		valueBytes[2] = grabValue[0]
		valueToConvert := [4]byte{0x00, grabValue[2], grabValue[1], grabValue[0]}
		secondSystem.System.Status = true
		secondSystem.System.P2 = float32(convert.ToLong(valueToConvert)) * 0.0001
	}
}

func (secondSystem *SecondSystem) populateTime() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = secondSystem.grabber.GrabValueBytes([]byte{0x84, 0x80, 0x40, 0x24}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		secondSystem.System.Status = true
		secondSystem.System.TimeRunSys = convert.ToLong(valueBytes)
	}
}
