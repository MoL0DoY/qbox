package systems

import (
	"qBox/drivers/skm2/data"
	"qBox/models"
	"qBox/services/convert"
	"time"
)

type Common struct {
	grabber    data.Grabber
	DataDevice *models.DataDevice
}

func (common *Common) PopulateFromBytes(b []byte) {
	common.grabber = data.Grabber{Datum: b}
	common.populateTimeRunCommon()
	common.populateTimeOn()
	common.populateTimeOnDevice()
}

func (common *Common) populateTimeRunCommon() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = common.grabber.GrabValueBytes([]byte{0x04, 0x24}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		(*common.DataDevice).TimeRunCommon = convert.ToLong(valueBytes)
	}
}

func (common *Common) populateTimeOn() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = common.grabber.GrabValueBytes([]byte{0x04, 0x20}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		valueBytes[0] = grabValue[3]
		valueBytes[1] = grabValue[2]
		valueBytes[2] = grabValue[1]
		valueBytes[3] = grabValue[0]
		(*common.DataDevice).TimeOn = convert.ToLong(valueBytes)
	}
}

func (common *Common) populateTimeOnDevice() {
	var valueBytes [4]byte
	var grabValue []byte
	grabValue = common.grabber.GrabValueBytes([]byte{0x04, 0x6D}, len(valueBytes))
	if cap(valueBytes) == len(grabValue) {
		year := 2000 + int(grabValue[2]>>5) + (int(grabValue[3]&0xF0) >> 1)
		month := time.Month(int(grabValue[3] & 0x0F))
		day := int(grabValue[2] & 0x1F)
		hour := int(grabValue[1])
		min := int(grabValue[0])
		(*common.DataDevice).Time = time.Date(year, month, day, hour, min, 0, 0, time.Local)
	}
}
