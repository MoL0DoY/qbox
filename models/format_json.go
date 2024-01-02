package models

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

type JsonFormat struct {
}

func (format JsonFormat) Render(writer io.Writer, device *DataDevice) {
	deviceForJson := dataDeviceJson{
		Serial:        device.Serial,
		UnitQ:         device.UnitQ,
		TimeRequest:   JSONTime(device.TimeRequest),
		Time:          JSONTime(device.Time),
		TimeOn:        device.TimeOn,
		TimeRunCommon: device.TimeRunCommon,
	}

	for _, system := range device.Systems {
		if system.Status == false {
			continue
		}
		deviceForJson.Systems = append(deviceForJson.Systems, systemDeviceJson(system))
	}

	bytesResponse, err := json.Marshal(deviceForJson)
	if err != nil {
		fmt.Fprintln(writer, "{}")
	}
	fmt.Fprintln(writer, string(bytesResponse))
}

/**
Чтобы не засорять код файла device.go подробностями по JSON,
решено сделать дубликат структур с настройками под JSON формат.
*/
type dataDeviceJson struct {
	Serial        string             `json:"serial"`
	UnitQ         UnitQEnum          `json:"unitQ"`
	TimeRequest   JSONTime           `json:"timeRequest"`
	Time          JSONTime           `json:"timeDevice"`
	TimeOn        uint32             `json:"timeOn"`
	TimeRunCommon uint32             `json:"timeRunCommon"`
	Systems       []systemDeviceJson `json:"system"`
}

type systemDeviceJson struct {
	TimeRunSys uint32 `json:"timeRunSys"`
	SigmaQ     float64
	Q1         float64
	Q2         float64
	Q3         float64
	V1         float64
	V2         float64
	M1         float64
	M2         float64
	GM1        float32
	GM2        float32
	GV1        float32
	GV2        float32
	T1         float32
	T2         float32
	T3         float32
	P1         float32
	P2         float32
	P3         float32
	Status     bool `json:"-"`
}

type JSONTime time.Time

// Конвертация формата time.Time к UnixTime
func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprint(time.Time(t).Unix())
	return []byte(stamp), nil
}
