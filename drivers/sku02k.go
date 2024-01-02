package drivers

import (
	"bytes"
	"encoding/hex"
	"qBox/drivers/skm2/data"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

/*
Драйвер для опроска SKU-02К теплосчётчиков
Всегда имеет одну систему.
*/
type SKU02K struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

/**
counterNumber для SKU-02K:
254 (0xFE) - воспринимается всеми теплосчетчиками, вне зависимости от их адресов.
1-250 - принадлежат ведомым теплосчётчикам.
*/
func (sku *SKU02K) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	sku.logger = logger
	sku.network = network
	sku.counterNumber = counterNumber
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (sku *SKU02K) Read() (*models.DataDevice, error) {

	sku.logger.Info("Запрос на инициализацию прибора, № %d", sku.counterNumber)
	request := net.PrepareRequest([]byte{
		0x10,
		0x40, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x40, sku.counterNumber}),
		0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err := sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос на инициализацию программного уровня протокола M-Bus")
	request = net.PrepareRequest([]byte{
		0x68, 0x03, 0x03, 0x68,
		0x73, sku.counterNumber, 0x50,
		sku.calculateCheckSum([]byte{0x73, sku.counterNumber, 0x50}), 0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос типа прибора и единиц измерения")
	request = net.PrepareRequest([]byte{
		0x68, 0x06, 0x06, 0x68,
		0x73, sku.counterNumber,
		0x51, 0x08, 0xFF, 0x0C,
		sku.calculateCheckSum([]byte{0x73, sku.counterNumber, 0x51, 0x08, 0xFF, 0x0C}), 0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	var response []byte

	sku.logger.Info("Запрос на просмотр ответа")
	request = net.PrepareRequest([]byte{0x10, 0x7B, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x7B, sku.counterNumber}), 0x16})
	request.ControlFunction = sku.checkConfigDeviceResponse
	request.SecondsReadTimeout = 7
	response, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}
	// вариант ответа, в нем A-адрес M-bus, ID-заводской номер, Vrs-номер протокола (06)
	// Md-модификация (0Dh-for heat/cold energy), TC-номер ответа, StSign-meter status code
	// 01h FFh-DIF VIF идиницы Q (0C 02), CS-control sum
	// 68 13 13 68 08 02 72 69 71 00 00 09 07 06 0D BA 00 00 00 01 FF 0C 02 41 16
	// 68h L L 68h C  A  CI ID Man      AXL  Vrs Md TC St Sign  DIF VIF Data CS 16h
	// L - длинна от 68h до CS
	// C - control fild (40h-инициализация/сброс, 08h-ответ от слэйва, 53h/73h-передача о мастера,
	// 5Ah/7Ah-запрос от мастера UD-1, 5Bh/7Bh-запрос от мастера UD-2)
	// A - adress
	// CI - control information fild (50h-reset, 51h-передача от мастере, 52h-выбор устройства, 72h-ответ от слэйва,
	// B8h/B9h/BAh/BBh/BCh/BDh-установка скорости 300-9600 бод)
	// CS - контрольная сумма
	// Received QALCOSONIC HEAT1 (SKU-03) device ID=7169 (Manufacture AXI) data

	// номер прибора в сети M-Bus
	sku.logger.Info("Получен адрес счетчика: %d", int(response[5]))

	sku.logger.Info("Запрос на инициализацию программного уровня протокола M-Bus")
	request = net.PrepareRequest([]byte{
		0x68, 0x03, 0x03, 0x68,
		0x73, sku.counterNumber, 0x50,
		sku.calculateCheckSum([]byte{0x73, sku.counterNumber, 0x50}), 0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	// запрос на формирование нужных данных из таблицы 11
	// состоит из последовательности запрашиваемых parameters
	// 68h L L 68h 73h  (53h) A 51h SEL1 SEL2 … SELN CS 16h
	sku.logger.Info("Запрос на получение данных")

	userData := []byte{
		0x73, sku.counterNumber, 0x51,
		0xC8, 0xFF, 0x7F, 0x6D, // Date and time stamp, (F)
		0xC8, 0xFF, 0x7F, 0x24, // Working time without error (sec)
		0xC8, 0x0F, 0xFE, 0x3B, // Energy for heating (MWh)
		0xC8, 0xFF, 0x7F, 0x13, // Volume (m3)
		0xC8, 0xFF, 0x7F, 0x3E, // Averago Flow rate (m3/h)
		0xC8, 0xFF, 0x7F, 0x5B, // Average Temperature 1 (ºC)
		0xC8, 0xFF, 0x7F, 0x5F, // Average Temperature 2 (ºC)
		0xC8, 0xFF, 0x7F, 0xFF, 0x0C, // Energy Unit Index
	}
	length := uint8(len(userData))
	headerBytes := []byte{
		0x68, length, length, 0x68,
	}
	checksum := []byte{sku.calculateCheckSum(userData)}

	request = net.PrepareRequest(bytes.Join([][]byte{headerBytes, userData, checksum, []byte{0x16}}, []byte("")))
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос на просмотр ответа")

	request = net.PrepareRequest([]byte{0x10, 0x7B, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x7B, sku.counterNumber}), 0x16})
	request.ControlFunction = sku.checkLongFrame
	request.SecondsReadTimeout = 7
	response, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.data.TimeRequest = time.Now()
	sku.data.Serial = hex.EncodeToString([]byte{response[10], response[9], response[8], response[7]})

	// У прошивки sku03 нет текущий температур и расходов, только часовые, суточные, месячные
	sku.logger.Info("Запрос на просмотр суточных")
	userData = []byte{0x73, sku.counterNumber, 0x50, 0x30}
	headerBytes = []byte{0x68, 0x04, 0x04, 0x68}
	checksum = []byte{sku.calculateCheckSum(userData)}
	request = net.PrepareRequest(bytes.Join([][]byte{headerBytes, userData, checksum, []byte{0x16}}, []byte("")))
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос на получение суточных данных")
	userData = []byte{0x73, sku.counterNumber, 0x51,
		0xC8, 0xFF, 0x7F, 0x3E, // Averago Flow rate (m3/h)
		0xC8, 0xFF, 0x7F, 0x5B, // Average Temperature 1 (ºC)
		0xC8, 0xFF, 0x7F, 0x5F, // Average Temperature 2 (ºC)
	}
	length = uint8(len(userData))
	headerBytes = []byte{0x68, length, length, 0x68}
	checksum = []byte{sku.calculateCheckSum(userData)}
	request = net.PrepareRequest(bytes.Join([][]byte{headerBytes, userData, checksum, []byte{0x16}}, []byte("")))
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос на просмотр ответа")

	// Определено:
	// Когда опрашиваются например суточные значения, первый запрос посылается с 7b
	// Чтобы увидеть данные за предыдущие сутки нужно послать 5b
	// Чтобы увидеть данные за предпредущие сутки опять нужно послать 7b и так далее чередуем 7b - 5b - 7b - 5b
	// Заводская программа посылает сначала всегда 7b
	request = net.PrepareRequest([]byte{0x10, 0x7B, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x7B, sku.counterNumber}), 0x16})
	request.ControlFunction = sku.checkLongFrame
	request.SecondsReadTimeout = 7
	responseForDay, err := sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.applyResponse(response, responseForDay)

	return &sku.data, nil
}

/**
Проверка контрольной суммы для SKU-02K
Представляет собой сумму значений из bytes, урезенную до одного байта
*/
func (sku *SKU02K) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return sum & 0xFF
}

// прибор может быть только односистемный однопоточный, конф. U1 или U2

func (sku *SKU02K) applyResponse(datum []byte, datumForDay []byte) {
	sku.data.AddNewSystem(1)
	sku.data.Systems[0].Status = true

	// принят Energy Unit Index 16 bit
	// cursor = 84
	// префикс 01 FF

	grabber := data.Grabber{Datum: datum}
	grabberForDay := data.Grabber{Datum: datumForDay}
	result := grabber.GrabValueBytes([]byte{0x01, 0xFF}, 1)

	factor := float32(0.1)

	// Q в ГДж или MWh предположение на основе протокола SKU-02K
	switch result[0] {
	case 0x0E:
		sku.logger.Debug("Q в Gj")
		sku.data.UnitQ = models.GJ
		factor = float32(0.001)
		break
	case 0x06:
		sku.logger.Debug("Q в MWh")
		sku.data.UnitQ = models.MWh // или КВт
		factor = float32(0.001)
		break
	case 0x05:
		sku.logger.Debug("Q в MWh")
		sku.data.UnitQ = models.MWh
		factor = float32(0.0001)
		break
	case 0x0D:
		sku.logger.Debug("Q в Gj")
		sku.data.UnitQ = models.MWh
		factor = float32(0.0001)
		break
	case 0x07:
		sku.logger.Debug("Q в MWh")
		sku.data.UnitQ = models.MWh
		factor = float32(0.01)
		break
	case 0x0F:
		sku.logger.Debug("Q в Gj")
		sku.data.UnitQ = models.MWh
		factor = float32(0.01)
		break
	default:
		sku.logger.Debug("Q в Gj")
		sku.data.UnitQ = models.GJ
		factor = float32(0.001)
		break
	}

	result = grabber.GrabValueBytes([]byte{0x04, 0x86, 0x3B}, 4)
	if 4 == len(result) {
		// Для прошивки как SKU-04
		sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]})) * factor)
		sku.data.Systems[0].SigmaQ = sku.data.Systems[0].Q1
	} else {
		result = grabber.GrabValueBytes([]byte{0x04, 0x8E, 0x3B}, 4)
		if 4 == len(result) {
			// Для прошивки как QALCOSONIC HEAT1 SKU-03
			sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
				result[3],
				result[2],
				result[1],
				result[0]})) * factor)
			sku.data.Systems[0].SigmaQ = sku.data.Systems[0].Q1
		} else {
			sku.logger.Info("Не найдены байты для Q1")
		}
	}

	// factor = float32(0.001)
	result = grabber.GrabValueBytes([]byte{0x04, 0x13}, 4)
	if 4 == len(result) {
		sku.data.Systems[0].V1 = float64(float32(ToLong([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]})) * factor)
	} else {
		sku.logger.Info("Не найдены байты для V1")
	}

	// расшифровка Date and time stamp
	result = grabber.GrabValueBytes([]byte{0x04, 0x6D}, 4)
	if 4 == len(result) {
		year := 2000 + int(result[2]>>5) + (int(result[3]&0xF0) >> 1)
		month := time.Month(int(result[3] & 0x0F))
		day := int(result[2] & 0x1F)
		hour := int(result[1])
		min := int(result[0])
		sku.data.Time = time.Date(year, month, day, hour, min, 0, 0, time.Local)
	} else {
		sku.logger.Info("Не найдены байты для Даты Время")
	}

	// расшифровка Working time without error в секундах
	result = grabber.GrabValueBytes([]byte{0x04, 0x24}, 4)
	if 4 == len(result) {
		sku.data.TimeOn = ToLong([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]})
	} else {
		sku.logger.Info("Не найдены байты для TimeOn")
	}

	// расшифровка Date and time of  error starting
	result = grabber.GrabValueBytes([]byte{0x34, 0x6D}, 4)
	if 4 == len(result) {
		sku.data.TimeRunCommon = ToLong([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]})
		sku.data.Systems[0].TimeRunSys = sku.data.TimeRunCommon
	} else {
		sku.logger.Info("Не найдены байты для TimeRunCommon")
	}

	// G
	factor = float32(1.0)
	result = grabber.GrabValueBytes([]byte{0x05, 0x3E}, 4)
	if 4 == len(result) {
		sku.data.Systems[0].GV1 = ToFloat([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]}) * factor
	} else {
		// Данные могут лежать в суточных
		result = grabberForDay.GrabValueBytes([]byte{0x85, 0x08, 0x3E}, 4)
		if 4 == len(result) {
			sku.data.Systems[0].GV1 = ToFloat([4]byte{
				result[3],
				result[2],
				result[1],
				result[0]}) * factor
		} else {
			sku.logger.Info("Не найдены байты для GV1")
		}
	}

	factorT := float32(1.0)

	// T1
	result = grabber.GrabValueBytes([]byte{0x05, 0x5B}, 4)
	if 4 == len(result) {
		sku.data.Systems[0].T1 = ToFloat([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]}) * factorT
	} else {
		// Данные могут лежать в суточных
		result = grabberForDay.GrabValueBytes([]byte{0x85, 0x08, 0x5B}, 4)
		if 4 == len(result) {
			sku.data.Systems[0].T1 = ToFloat([4]byte{
				result[3],
				result[2],
				result[1],
				result[0]}) * factor
		} else {
			sku.logger.Info("Не найдены байты для T1")
		}
	}

	// T2
	factor = float32(1.0)
	result = grabber.GrabValueBytes([]byte{0x05, 0x5F}, 4)
	if 4 == len(result) {
		sku.data.Systems[0].T2 = ToFloat([4]byte{
			result[3],
			result[2],
			result[1],
			result[0]}) * factorT
	} else {
		// Данные могут лежать в суточных
		result = grabberForDay.GrabValueBytes([]byte{0x85, 0x08, 0x5F}, 4)
		if 4 == len(result) {
			sku.data.Systems[0].T2 = ToFloat([4]byte{
				result[3],
				result[2],
				result[1],
				result[0]}) * factor
		} else {
			sku.logger.Info("Не найдены байты для T2")
		}
	}
	return
}

func (sku *SKU02K) checkSimpleFrame(response []byte) bool {
	if len(response) == 0 {
		sku.logger.Info("Получен пустой ответ.")
		return false
	}

	if len(response) > 0 && response[0] == 0xE5 {
		return true
	} else {
		sku.logger.Info("Получен некорректный ответ. Проверка SimpleFrame не пройдена.")
		return false
	}
}

func (sku *SKU02K) checkConfigDeviceResponse(response []byte) bool {
	if len(response) < 5 {
		sku.logger.Info("Получен некорректный ответ.")
		return false
	}

	return true
}

func (sku *SKU02K) checkLongFrame(response []byte) bool {
	if len(response) < 19 {
		// Структура ответа на запрос текущих данных занимает от 19 байт и больше
		sku.logger.Info("Получен некорректный ответ. Ответ содержит меньше 19 байт.")
		return false
	}

	if response[0] != response[3] || response[0] != 0x68 || response[1] != response[2] {
		sku.logger.Info(
			"Получен некорректный ответ. Заголовок(68h L L 68h) ответа не верный - %X",
			response[0:3])
		return false
	}

	L := response[1] // задаём L из заголовка

	if len(response) < int(L)+4+1+1 { // L + Заголовок(68h L L 68h) + CheckSum + Stop(16h)
		sku.logger.Info("Получен некорректный ответ."+
			" Некорректная длинна ответа, согласно заголовку (68h L L 68h)."+
			" Длина запроса - %d"+
			" Ожидалось - %d", len(response), int(L)+4+1+1)
		return false
	}

	checkSum := response[len(response)-2]
	calculatedCheckSum := sku.calculateCheckSum(response[4 : len(response)-2])
	if calculatedCheckSum != checkSum {
		sku.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		sku.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		sku.logger.Debug("Получена контрольная сумма- %X", checkSum)
		return false
	}

	return true
}
