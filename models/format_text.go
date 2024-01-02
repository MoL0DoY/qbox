package models

import (
	"fmt"
	"io"
)

type TextFormat struct {
}

func (format TextFormat) Render(writer io.Writer, device *DataDevice) {
	fmt.Fprintf(writer, "Заводской номер прибора - %v\n", device.Serial)
	fmt.Fprintf(writer, "Время опроса - %s\n", device.TimeRequest.Format("02.01.2006 15:04:05"))
	fmt.Fprintf(writer, "Время на приборе - %s\n", device.Time.Format("02.01.2006 15:04:05"))
	fmt.Fprintf(writer, "Время работы при включенном питании - %f ч\n", float32(device.TimeOn)/3600.00)
	fmt.Fprintf(writer, "Время работы без ошибок - %f ч\n", float32(device.TimeRunCommon)/3600.00)

	var textUnitQ string

	switch device.UnitQ {
	case MWh:
		textUnitQ = "МВт"
	case KWh:
		textUnitQ = "КВт"
	case GJ:
		textUnitQ = "ГДж"
	case Gcal:
		textUnitQ = "ГКал"
	}
	for i, system := range device.Systems {
		if system.Status == false {
			continue
		}
		fmt.Fprintln(writer, "")
		fmt.Fprintf(writer, "Показания системы %d:\n", i+1)
		fmt.Fprintf(writer, "Q результирующее %f %s\n", system.SigmaQ, textUnitQ)
		fmt.Fprintf(writer, "Q1 %f %s\n", system.Q1, textUnitQ)
		fmt.Fprintf(writer, "Q2 %f %s\n", system.Q2, textUnitQ)
		fmt.Fprintf(writer, "Q3 %f %s\n", system.Q3, textUnitQ)
		fmt.Fprintf(writer, "V1 %f м3\n", system.V1)
		fmt.Fprintf(writer, "V2 %f м3\n", system.V2)
		fmt.Fprintf(writer, "M1 %f тонн\n", system.M1)
		fmt.Fprintf(writer, "M2 %f тонн\n", system.M2)
		fmt.Fprintf(writer, "G1 массовый %f тонн/ч\n", system.GM1)
		fmt.Fprintf(writer, "G2 массовый %f тонн/ч\n", system.GM2)
		fmt.Fprintf(writer, "G1 объёмный %f м3/ч\n", system.GV1)
		fmt.Fprintf(writer, "G2 объёмный %f м3/ч\n", system.GV2)
		fmt.Fprintf(writer, "T1 %f C\n", system.T1)
		fmt.Fprintf(writer, "T2 %f C\n", system.T2)
		fmt.Fprintf(writer, "T3 %f C\n", system.T3)
		fmt.Fprintf(writer, "P1 %f МПа\n", system.P1)
		fmt.Fprintf(writer, "P2 %f МПа\n", system.P2)
		fmt.Fprintf(writer, "P3 %f МПа\n", system.P3)
		fmt.Fprintf(writer, "Время работы системы (без ошибок) № %d - %f ч\n", i+1, float32(system.TimeRunSys)/3600.00)
	}

	fmt.Fprintln(writer, "")
}
