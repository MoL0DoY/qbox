package drivers

import (
	"errors"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

/*
Драйвер для опроса SKU-02 теплосчётчиков
Версия 1.0.0
*/
type SKU02 struct {
	data    models.DataDevice
	network *net.Network
	logger  *log.LoggerService
}

/**
При запросе номер счётчика не передаётся
Инициализации как такой нет, т.к. в Read() совместно с опросом текущих удаётся получить всю техническую информацию.
Возможно в будущем, при оптимизации стоит сюда что-то перенести.
*/
func (sku *SKU02) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	sku.logger = logger
	sku.network = network
	//Система всегда одна
	sku.data.AddNewSystem(1)
	sku.data.Systems[0].Status = true
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (sku *SKU02) Read() (*models.DataDevice, error) {

	sku.logger.Info("Запрос текущих данных")
	request := net.PrepareRequest(createRequest(0x20))
	request.ControlFunction = sku.checkFrame
	response, err := sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}
	sku.populate(response)

	sku.logger.Info("Запрос на чтение даты времени")
	sku.data.TimeRequest = time.Now()
	/**
	При каждом запросе в шапке данных(с 25 по 28 байт) содержится текущее время счётчика, но структура не содержит минуты.
	Отсутствие минут, секунд критично. Поэтому делается отдельный запрос с командой 0x28. В ответе содержится
	текущее время счётчика с минутами и секундами.
	*/
	request = net.PrepareRequest(createRequest(0x28))
	request.ControlFunction = sku.checkFrame
	response, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	if len(response) < 36 {
		return &sku.data, errors.New("получены некорректные данные")
	}
	sku.data.Time = time.Date(
		int(response[30])*0x100+int(response[31]),
		time.Month(response[32]),
		int(response[33]),
		int(response[34]),
		int(response[35]),
		int(response[36]),
		0,
		time.Local)

	return &sku.data, nil
}

/**
Получение ответа от счётчика с проверкой на корректность результата.
*/
func (sku *SKU02) checkFrame(response []byte) bool {

	if len(response) < 4 {
		// Заголовок ещё не получен.
		sku.logger.Info("Получен некорректный ответ. Ответ содержит меньше 4 байт.")
		return false
	}

	if response[0] != response[3] || response[0] != 0x68 {
		sku.logger.Info("Заголовок(68h blockLen 68h) ответа неверный - %X", response[0:3])
		return false
	}

	blockLen := toWord([2]byte{response[1], response[2]}) // задаём L из заголовка

	if len(response) < int(blockLen) {
		sku.logger.Info("Получен некорректный ответ.")
		return false
	}

	if response[blockLen-1] != 0x16 {
		sku.logger.Info("Стоповый бит неверный - %X", response[blockLen-1])
		return false
	}
	if !sku.checkCRC(response, blockLen) {
		sku.logger.Info("Контрольная сумма неверная - ")
		return false
	}

	return true
}

/**
Заполнение SKU данными
Каждый ответ от счётчика содержит 29 байт служебных данных, описанных в протоколе(именуется там "шапка")
С 30 байта идут "запрашиваемые" данные.
*/
func (sku *SKU02) populate(datum []byte) {

	sku.data.Serial = strconv.FormatUint(uint64(ToLong([4]byte{datum[21], datum[22], datum[23], datum[24]})), 10)
	sku.data.TimeOn = calculateLongByPointer(datum, 46)

	/*
		Привет мутным Димену и Даугу ;)

		В 18, 19 и 20 байтах лежат единицы измерения Q(энергии), P(давления), V или M (воды)

		Структура из протокола:
		18 байт - это индекс для {"MWh ", "Gcal", "GJ"}
		19 байт - это индекс для {"kPa ", "MPa ", "MPa"}
		20 байт - это индекс для {"m3 ", "t ", "t"}, т.е. Объём это или Масса.
		Соответственно расходы будут в {"m3/h", "t/h ", "t/h "}

		В протоколе встречается	массив из {0.001, 0.000859845, 0.0036} и {1.0, 0.001, 0.001}:
		Первое - это поправочные коэффициенты для полученных значений энергии в запросе (нужно умножить).
		Второе - это поправочные коэффициенты для полученных значений давлений в запросе (нужно умножить).

		В протоколе также встречается структура:
		[]float32{
		1.0, 1.0, 1.0, 1.0,
		10.0, 10.0, 10.0, 10.0,
		100.0, 100.0, 100.0, 100.0,
		100.0, 100.0, 100.0, 100.0}

		Это поправочные коэффициенты размерности(dimension) для полученных значений Энергии(Q) и Воды(V и М) (нужно умножить).
		Размерность зависит от диаметра выставленного в настройках счётчика.
	*/

	dimension := []float32{
		1.0, 1.0, 1.0, 1.0,
		10.0, 10.0, 10.0, 10.0,
		100.0, 100.0, 100.0, 100.0,
		100.0, 100.0, 100.0, 100.0}[int(datum[18]&0xF0)>>4]

	sku.data.Systems[0].SigmaQ = float64(float32(calculateLongByPointer(datum, 50)))
	sku.data.Systems[0].Q1 = float64(float32(calculateLongByPointer(datum, 30)))
	sku.data.Systems[0].Q2 = float64(float32(calculateLongByPointer(datum, 34)))

	switch int(datum[18] & 0x0F) {
	case 0:
		sku.data.UnitQ = models.MWh
		sku.data.Systems[0].SigmaQ *= 0.001 * float64(dimension)
		sku.data.Systems[0].Q1 *= 0.001 * float64(dimension)
		sku.data.Systems[0].Q2 *= 0.001 * float64(dimension)
	case 1:
		sku.data.UnitQ = models.Gcal
		sku.data.Systems[0].SigmaQ *= 0.000859845 * float64(dimension)
		sku.data.Systems[0].Q1 *= 0.000859845 * float64(dimension)
		sku.data.Systems[0].Q2 *= 0.000859845 * float64(dimension)
	case 2:
		sku.data.UnitQ = models.GJ
		sku.data.Systems[0].SigmaQ *= 0.0036 * float64(dimension)
		sku.data.Systems[0].Q1 *= 0.0036 * float64(dimension)
		sku.data.Systems[0].Q2 *= 0.0036 * float64(dimension)
	}

	switch int(datum[20] & 0x0F) {
	case 0:
		sku.data.Systems[0].V1 = float64(float32(calculateLongByPointer(datum, 38)) * 0.01 * dimension)
		sku.data.Systems[0].V2 = float64(float32(calculateLongByPointer(datum, 42)) * 0.01 * dimension)
		sku.data.Systems[0].GV1 = calculateFloatByPointer(datum, 70)
		sku.data.Systems[0].GV2 = calculateFloatByPointer(datum, 74)
		break

	case 2:
		sku.data.Systems[0].M1 = float64(float32(calculateLongByPointer(datum, 38)) * 0.01 * dimension)
		sku.data.Systems[0].M2 = float64(float32(calculateLongByPointer(datum, 42)) * 0.01 * dimension)
		sku.data.Systems[0].GM1 = calculateFloatByPointer(datum, 70)
		sku.data.Systems[0].GM2 = calculateFloatByPointer(datum, 74)
		break
	default:
		sku.data.Systems[0].M1 = float64(float32(calculateLongByPointer(datum, 38)) * 0.01 * dimension)
		sku.data.Systems[0].M2 = float64(float32(calculateLongByPointer(datum, 42)) * 0.01 * dimension)
		sku.data.Systems[0].GM1 = calculateFloatByPointer(datum, 70)
		sku.data.Systems[0].GM2 = calculateFloatByPointer(datum, 74)
		break
	}

	sku.data.Systems[0].T1 = calculateFloatByPointer(datum, 86)
	sku.data.Systems[0].T2 = calculateFloatByPointer(datum, 90)
	sku.data.Systems[0].T3 = calculateFloatByPointer(datum, 94)

	sku.data.Systems[0].P1 = calculateFloatByPointer(datum, 98)
	sku.data.Systems[0].P2 = calculateFloatByPointer(datum, 102)

	if int(datum[19]&0x0F) == 0 { // значение передано в килоПаскалях
		sku.data.Systems[0].P1 *= 0.001
		sku.data.Systems[0].P2 *= 0.001
	}
}

// Формирование запроса на отправку команды
func createRequest(command byte) []byte {
	var request = [255]byte{}
	lengthRequest := 34

	request[0] = 0x68                                             //первый стартовый 0x86					|byte
	request[1] = 0                                                //Длинна блока данных					|word  Старший
	request[2] = byte(lengthRequest)                              //Длинна блока данных					|word  Младший
	request[3] = 0x68                                             //Второй стартовый 0x86					|byte
	request[4] = 0                                                //тип прибора (SKU-02 , 0x02)			|word   Старший
	request[5] = 0x02                                             //тип прибора (SKU-02 , 0x02)			|word   Младший
	request[6] = 0                                                //Версия прибора						| float
	request[7] = 0                                                //Версия прибора						| float
	request[8] = 0                                                //Версия прибора						| float
	request[9] = 0                                                //Версия прибора						| float
	request[10] = 0                                               //Статус данных		                	|byte
	request[11] = 0                                               //Номер блока передаваемых данных		|word   Старший
	request[12] = 0                                               //Номер блока передаваемых данных		|word   Младший
	request[13] = 0                                               //Полная длинна передаваемых данных		|long   Старший
	request[14] = 0                                               //Полная длинна передаваемых данных		|long   Старший
	request[15] = 0                                               //Полная длинна передаваемых данных		|long   Младший
	request[16] = 0                                               //Полная длинна передаваемых данных		|long   Младший
	request[17] = 0                                               //Модификация							|byte
	request[18] = 0                                               //DimenE								|byte
	request[19] = 0                                               //UnitP									|byte
	request[20] = 0                                               //FDim									|byte
	request[21] = 0                                               //номер прибора							|long   Старший
	request[22] = 0                                               //номер прибора							|long   Старший
	request[23] = 0                                               //номер прибора							|long   Младший
	request[24] = 0                                               //номер прибора							|long   Младший
	request[25] = 0                                               //реальное время прибора	год			|signed char
	request[26] = 0                                               //реальное время прибора	Месяц		|signed char
	request[27] = 0                                               //реальное время прибора	День		|signed char
	request[28] = 0                                               //реальное время прибора	Час			|signed char
	request[29] = command                                         //Соманда								|byte
	controlSum1 := &request[lengthRequest-4 : lengthRequest-3][0] //Контрольная сумма1					|byte
	controlSum2 := &request[lengthRequest-3 : lengthRequest-2][0] //Контрольная сумма2					|byte
	controlSum3 := &request[lengthRequest-2 : lengthRequest-1][0] //Контрольная сумма3					|byte
	bStop := &request[lengthRequest-1 : lengthRequest][0]         //Стоп									|byte

	*controlSum1 = 0
	*controlSum2 = 0
	*controlSum3 = 0
	*bStop = 0x16

	for i := 0; i < int(lengthRequest-4); i++ {
		*controlSum1 ^= request[i]
		*controlSum3 += request[i]
	}

	*controlSum3 += *controlSum1 * 2
	*controlSum2 = *controlSum3 ^ 0xFF

	return request[:lengthRequest]
}

/*
Проверка контрольной суммы
buffer - должен быть передан полностью весь запрос, со всеми заголовками. Заголовок(68h L 68h) + DATA + CheckSums + Stop(16h)
length - длина "полезных" данных (L)
*/
func (sku *SKU02) checkCRC(buffer []byte, length uint16) bool {

	var (
		controlSum1 byte = 0
		controlSum2 byte = 0
		controlSum3 byte = 0
	)

	for i := 0; i < int(length)-4; i++ {
		controlSum1 ^= buffer[i]
		controlSum3 += buffer[i]
	}
	controlSum3 += controlSum1 * 2
	controlSum2 = controlSum3 ^ 0xFF

	return controlSum1 == buffer[length-4] && controlSum2 == buffer[length-3] && controlSum3 == buffer[length-2]
}
