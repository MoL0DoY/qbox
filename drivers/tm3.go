package drivers

import (
	"errors"
	"github.com/npat-efault/crc16"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

// Преобразователь измерительный многофункциональный "ИСТОК-ТМ3", НПЦ "Спецсистема"
// Протокол обмена ModBus RTU
// Версия 0.0.1
type TM3 struct {
	data    models.DataDevice
	network *net.Network
	logger  *log.LoggerService
	number  byte

	/*
		Коэф ед. давления
	*/
	coefficientP float32

	/*
		Коэф. расхода, воды
	*/
	coefficientV float32
}

/**
 */
func (tm3 *TM3) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	var response []byte
	var err error

	tm3.logger = logger
	tm3.network = network
	tm3.number = counterNumber
	tm3.logger.Info("Инициализация прибора, № %d", tm3.number)

	tm3.logger.Info("Запрос серийного номера прибора")
	/**
	Сейчас заводской номер складывается из даты производства (год + месяц) и уникального номера в партии.
	Таким образом, 1612001 - год 16, месяц 12, номер 001.
	Необходимо вычитывать регистры EF04-EF08
	32х разрядное в регистрах 0xEF04-0xEF05
	структура регистра 0xEF07
	struct
	{
		const uint16_t day : 5;
		const uint16_t month: 4;
		const uint16_t year : 7;
	};
	*/
	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xEF, 0x04, 0x00, 0x04})
	for err != nil {
		return err
	}

	ef07 := toWord([2]byte{response[6], response[7]})
	year := ef07 >> 0x09
	month := ef07 >> 0x05 & 0x0F

	logger.Debug("Год - %d", year)
	serial := strconv.FormatUint(uint64(year), 10)

	logger.Debug("Месяц - %d", month)
	if uint64(month) < 10 {
		serial += "0"
	}
	serial += strconv.FormatUint(uint64(ef07>>0x05&0x0F), 10)

	ef05 := uint64(toWord([2]byte{response[2], response[3]}))
	tm3.logger.Debug("Номер партии - %d", ef05)
	if (ef05) < 10 {
		serial += "00"
	} else if ef05 < 100 {
		serial += "0"
	}
	serial += strconv.FormatUint(ef05, 10)

	tm3.logger.Debug("Серийный номер - %s", serial)
	tm3.data.Serial = serial

	tm3.logger.Info("Запрос количества систем")
	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0x01, 0x43, 0x00, 0x01})
	for err != nil {
		return err
	}
	countSystem := int(toWord([2]byte{response[0], response[1]}))
	tm3.logger.Info("Количество система учёта - %d", countSystem)
	tm3.data.AddNewSystem(countSystem - 1)
	i := 0
	for i < countSystem {
		tm3.data.Systems[i].Status = true
		i++
	}

	tm3.logger.Info("Запрос единиц измерения энергии")

	tm3.data.UnitQ = models.Gcal

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x01, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitQ := int(toWord([2]byte{response[0], response[1]}))
	if unitQ == 1 {
		tm3.data.UnitQ = models.Gcal
	}
	if unitQ == 0 {
		tm3.data.UnitQ = models.GJ
	}

	tm3.logger.Info("Запрос единиц измерения давления")

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x00, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitP := int(toWord([2]byte{response[0], response[1]}))

	if unitP == 0 { // КПа
		tm3.coefficientP = 0.001
	} else if unitP == 1 { // кгс/см3
		tm3.coefficientP = 0.0980665
	} else if unitP == 2 { // бар
		tm3.coefficientP = 0.1
	} else if unitP == 3 { // МПа
		tm3.coefficientP = 1.0
	} else {
		tm3.logger.Debug("Ошибка при расшифровке ед. измерения давления: %d ", unitP)
		return errors.New("не определены единицы измерения давления")
	}

	logger.Info("Запрос единиц измерения объёма, массы")

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x02, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitV := int(toWord([2]byte{response[0], response[1]}))

	if unitV == 0 { // м3 или т
		tm3.coefficientV = 1.0
	} else if unitP == 1 { // тысячи м3 или тысячи тонн
		tm3.coefficientV = 0.001
	} else {
		tm3.logger.Debug("Ошибка при расшифровке ед. измерения воды: %d ", unitV)
		return errors.New("не определены единицы измерения воды")
	}

	return nil
}

/**
 */
func (tm3 *TM3) Read() (*models.DataDevice, error) {

	var response []byte
	var err error

	tm3.logger.Info("Запрос времени на приборе")
	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xEF, 0x50, 0x00, 0x02})
	for err != nil {
		return &tm3.data, err
	}

	tm3.data.TimeRequest = time.Now()

	t := [4]byte{response[0], response[1], response[2], response[3]}
	tm3.data.Time = time.Unix(int64(ToLong(t)), 0)

	i := 0

	for i < len(tm3.data.Systems) {

		tm3.data.Systems[i].Status = true

		tm3.logger.Info("Запрос данных для системы %d", i+1)
		response, err = tm3.runIO([]byte{tm3.number, 0x03, 0x70, byte(i * 4), 0x00, 0x3A})
		for err != nil {
			return &tm3.data, err
		}

		tm3.data.Systems[i].SigmaQ = float64(float32(toDouble(response[0:8]) / 1000000))

		tm3.data.Systems[i].Q1 = float64(float32(toDouble(response[8:16]) / 1000000))
		tm3.data.Systems[i].M1 = float64(float32(toDouble(response[16:24])) * 0.001)
		tm3.data.Systems[i].GM1 = calculateFloatByPointer(response, 24) * 0.001
		tm3.data.Systems[i].GV1 = calculateFloatByPointer(response, 28) * tm3.coefficientV
		tm3.data.Systems[i].T1 = calculateFloatByPointer(response, 32)
		tm3.data.Systems[i].P1 = calculateFloatByPointer(response, 36) * tm3.coefficientP

		if true {
			tm3.data.Systems[i].Q2 = float64(float32(toDouble(response[40:48]) / 1000000))
		} else {
			// По договорённости тут должно лежать Q2, но при работе счётчика в "замкнутом" режиме по каким-то причинам
			// не кладёт в этот адрес значение Q2. Значение лежит для первой системы в регистре 0x0480 в типе DOUBLE.
			// Решено, что если такая система установлена, то надо обращать внимание только на Q результирующее
			tm3.logger.Info("Запрос Q2 для замкнутой системы 1")
			responseQ2, err := tm3.runIO([]byte{tm3.number, 0x03, 0x04, 0x80, 0x00, 0x04})
			for err != nil {
				return &tm3.data, err
			}
			tm3.data.Systems[i].Q2 = float64(float32(toDouble(responseQ2) / 1000000))
		}

		tm3.data.Systems[i].M2 = float64(float32(toDouble(response[48:56]) * 0.001))
		tm3.data.Systems[i].GM2 = calculateFloatByPointer(response, 56) * 0.001
		tm3.data.Systems[i].GV2 = calculateFloatByPointer(response, 60) * tm3.coefficientV
		tm3.data.Systems[i].T2 = calculateFloatByPointer(response, 64)
		tm3.data.Systems[i].P2 = calculateFloatByPointer(response, 68) * tm3.coefficientP

		tm3.data.Systems[i].Q3 = float64(float32(toDouble(response[72:80]) / 1000000))

		/**
		Трубопровод подпитки. В ядре он не учтён.
		*/
		//M3 = float32(toDouble(response[80:88]) / 1000000)
		//GM3 = toFloat([4]byte{response[91], response[58], response[57], response[88]})
		//GV3 = toFloat([4]byte{response[95], response[62], response[61], response[92]})
		//температура подпитки= toFloat([4]byte{response[99], response[66], response[65], response[96]})
		//давление подпитки = toFloat([4]byte{response[103], response[70], response[69], response[100]})

		tm3.data.Systems[i].T3 = calculateFloatByPointer(response, 104)
		tm3.data.Systems[i].P3 = calculateFloatByPointer(response, 108) * tm3.coefficientP
		tm3.data.Systems[i].TimeRunSys = calculateLongByPointer(response, 112)

		i++
	}

	tm3.logger.Info("Запрос общего времени работы прибора")
	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xEF, 0x57, 0x00, 0x02})
	for err != nil {
		return &tm3.data, err
	}
	tm3.data.TimeOn = ToLong([4]byte{response[0], response[1], response[2], response[3]})
	return &tm3.data, nil

}

func (tm3 *TM3) checkResponse(response []byte) bool {

	if len(response) < 3 {
		tm3.logger.Info("Получен некорректный ответ. Ответ содержит меньше 3 байт.")
		return false
	}

	if response[0] != tm3.number || response[1] != 0x03 {
		tm3.logger.Info("modbus адрес прибора и функциональный код не совпадают в ответе")
		return false
	}

	calculatedCheckSum := intToLittleEndian(crc16.Checksum(crc16.Modbus, response[:len(response)-2]))
	checkSumResponse := response[len(response)-2:]
	if calculatedCheckSum[0] != checkSumResponse[0] || calculatedCheckSum[1] != checkSumResponse[1] {
		tm3.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		tm3.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		tm3.logger.Debug("Получена контрольная сумма- %X", checkSumResponse)
		return false
	}

	return true
}

func (tm3 *TM3) runIO(request []byte) ([]byte, error) {
	checkSum := intToLittleEndian(crc16.Checksum(crc16.Modbus, request))
	request = append(request, checkSum...)
	requestComponent := net.PrepareRequest(request)
	requestComponent.ControlFunction = tm3.checkResponse
	requestComponent.SecondsReadTimeout = 7
	response, err := tm3.network.RunIO(requestComponent)
	for err != nil {
		return nil, err
	}
	return response[3 : len(response)-2], nil
}
