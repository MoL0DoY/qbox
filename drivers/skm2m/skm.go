package skm2m

import (
	"encoding/hex"
	"qBox/drivers/skm2/data"
	"qBox/models"
	"qBox/services/convert"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

type SKM struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
	checks        data.Checks
}

/*
*
counterNumber для СКМ:
254 (0xFE) - воспринимается всеми теплосчетчиками, вне зависимости от их адресов.
0 адрес принадлежит несконфигурированным теплосчётчикам
1-250 - принадлежат ведомым теплосчётчикам.
*/
func (skm *SKM) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	skm.logger = logger
	skm.network = network
	skm.counterNumber = counterNumber
	skm.checks = data.Checks{Logger: logger}

	// Согласно переписке с производителем СКМ-2 счётчиков.
	// ПО верхнего уровня для преобразования использует коэффициент:
	// для МВт в ГКал 1.163(разделить), МВт в ГДж - 3.6(умножение)
	skm.data.CoefficientMWh = 1 / 1.163 // 0.85984522785
	skm.data.CoefficientKWh = 1 / 1.163 / 1000
	skm.data.CoefficientGJ = 1 / (3.6 / skm.data.CoefficientMWh) //0,238845896625

	skm.logger.Info("Запрос на инициализацию прибора, № %d", skm.counterNumber)
	request := net.PrepareRequest([]byte{
		0x10,
		0x40, skm.counterNumber,
		skm.checks.CalculateCheckSum([]byte{0x40, skm.counterNumber}),
		0x16})
	request.ControlFunction = skm.checks.CheckSimpleFrame
	_, err := skm.network.RunIO(request)
	return err
}

/*
*
Чтение текущих данных для СКМ-2 согласно протоколу M-bus EN 60870-5
*/
func (skm *SKM) Read() (*models.DataDevice, error) {

	skm.logger.Info("Запрос на чтение текущих данных")
	request := net.PrepareRequest([]byte{
		0x68, 0x04, 0x04, 0x68,
		0x53, skm.counterNumber,
		0x50, 0x10,
		skm.checks.CalculateCheckSum([]byte{0x53, skm.counterNumber, 0x50, 0x10}), 0x16})
	request.ControlFunction = skm.checks.CheckSimpleFrame
	_, err := skm.network.RunIO(request)
	if err != nil {
		return &skm.data, err
	}

	skm.data.TimeRequest = time.Now()

	skm.logger.Info("Запрос на просмотр ответа текущих данных")
	request = net.PrepareRequest([]byte{0x10, 0x5B, skm.counterNumber,
		skm.checks.CalculateCheckSum([]byte{0x5B, skm.counterNumber}), 0x16})
	request.ControlFunction = skm.checks.CheckLongFrame
	response1, err := skm.network.RunIO(request)
	for err != nil {
		return &skm.data, err
	}

	skm.logger.Info("Запрос на просмотр ответа текущих данных")
	request = net.PrepareRequest([]byte{0x10, 0x7B, skm.counterNumber,
		skm.checks.CalculateCheckSum([]byte{0x7B, skm.counterNumber}), 0x16})
	request.ControlFunction = skm.checks.CheckLongFrame
	response2, err := skm.network.RunIO(request)
	for err != nil {
		return &skm.data, err
	}

	skm.data.Serial = hex.EncodeToString([]byte{response1[10], response1[9], response1[8], response1[7]})

	skm.PopulateFromBytes(response1, response2)

	return &skm.data, nil
}

func (skm *SKM) PopulateFromBytes(b1 []byte, b2 []byte) {
	skm.data.Time = time.Date(2000+int(convert.ByteFromBDC(b1[24])), time.Month(int(convert.ByteFromBDC(b1[23]))), int(convert.ByteFromBDC(b1[22])), int(convert.ByteFromBDC(b1[21])), int(convert.ByteFromBDC(b1[20])), int(convert.ByteFromBDC(b1[19])), 0, time.Local)
	skm.data.TimeOn = convert.LongLittleEndianByPointer(b2, 177)

	skm.data.AddNewSystem(0)
	skm.data.AddNewSystem(1)

	if convert.LongLittleEndianByPointer(b2, 181) > 0 {
		skm.data.Systems[0].Status = true
		skm.data.Systems[0].TimeRunSys = convert.LongLittleEndianByPointer(b2, 181)
		skm.data.Systems[0].Q1 = float64(convert.LongLongLittleEndianByPointer(b1, 25)&0x0001FFFFFFFFFFFF) / 4.1868 * 1.163 / 1000000
		skm.data.Systems[0].T1 = convert.FloatLittleEndianByPointer(b2, 19)
		skm.data.Systems[0].T2 = convert.FloatLittleEndianByPointer(b2, 23)
		skm.data.Systems[0].T3 = convert.FloatLittleEndianByPointer(b2, 43)
		skm.data.Systems[0].P1 = convert.FloatLittleEndianByPointer(b2, 47)
		skm.data.Systems[0].P2 = convert.FloatLittleEndianByPointer(b2, 51)
		skm.data.Systems[0].P3 = convert.FloatLittleEndianByPointer(b2, 71)
		skm.data.Systems[0].V1 = float64(convert.LongLongLittleEndianByPointer(b1, 65)&0x00000000FFFFFFFF) / 100000
		skm.data.Systems[0].V2 = float64(convert.LongLongLittleEndianByPointer(b1, 73)&0x00000000FFFFFFFF) / 100000
		skm.data.Systems[0].M1 = float64(convert.LongLongLittleEndianByPointer(b1, 129)&0x00000000FFFFFFFF) / 100000
		skm.data.Systems[0].M2 = float64(convert.LongLongLittleEndianByPointer(b1, 137)&0x00000000FFFFFFFF) / 100000
		skm.data.Systems[0].GM1 = float32(convert.LongWordLittleEndianByPointer(b1, 197)) / 10000
		skm.data.Systems[0].GM2 = float32(convert.LongWordLittleEndianByPointer(b1, 205)) / 10000
		skm.data.Systems[0].GV1 = float32(convert.LongWordLittleEndianByPointer(b1, 193)) / 10000
		skm.data.Systems[0].GV2 = float32(convert.LongWordLittleEndianByPointer(b1, 201)) / 10000
		//skm.data.Systems[0].
	}

	if convert.LongLittleEndianByPointer(b2, 185) > 0 {
		skm.data.Systems[1].Status = true
		skm.data.Systems[1].TimeRunSys = convert.LongLittleEndianByPointer(b2, 185)
		skm.data.Systems[1].T1 = convert.FloatLittleEndianByPointer(b2, 27)
		skm.data.Systems[1].T2 = convert.FloatLittleEndianByPointer(b2, 31)
		skm.data.Systems[1].T3 = convert.FloatLittleEndianByPointer(b2, 43)
		skm.data.Systems[1].P1 = convert.FloatLittleEndianByPointer(b2, 55)
		skm.data.Systems[1].P2 = convert.FloatLittleEndianByPointer(b2, 59)
		skm.data.Systems[1].P3 = convert.FloatLittleEndianByPointer(b2, 71)
		skm.data.Systems[1].M1 = float64(convert.LongLongLittleEndianByPointer(b1, 145)&0x00000000FFFFFFFF) / 100000
		skm.data.Systems[1].Q1 = float64(convert.LongLongLittleEndianByPointer(b1, 33)&0x0001FFFFFFFFFFFF) / 4.1868 * 1.163 / 1000000
	}
}
