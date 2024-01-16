package drivers

import (
	"errors"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"

	"github.com/npat-efault/crc16"
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

	tm3.logger.Debug("Год - %d", year)
	tm3.logger.Debug("Месяц - %d", month)

	tm3.data.Serial = tm3.initSerial(year, month, ef07, response)
	tm3.logger.Debug("Серийный номер - %s", tm3.data.Serial)

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

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x01, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitQ := int(toWord([2]byte{response[0], response[1]}))
	switch unitQ {
	case 0:
		{
			tm3.data.UnitQ = models.GJ
			break
		}
	default:
		{
			tm3.data.UnitQ = models.Gcal
			break
		}
	}

	tm3.logger.Info("Запрос единиц измерения давления")

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x00, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitP := int(toWord([2]byte{response[0], response[1]}))
	switch unitP {
	case 0:
		{
			tm3.coefficientP = 0.001
			break
		}
	case 1:
		{
			tm3.coefficientP = 0.0980665
			break
		}
	case 2:
		{
			tm3.coefficientP = 0.1
			break
		}
	case 3:
		{
			tm3.coefficientP = 1.0
			break
		}
	default:
		{
			tm3.logger.Debug("Ошибка при расшифровке ед. измерения давления: %d ", unitP)
			return errors.New("не определены единицы измерения давления")
		}
	}
	logger.Info("Запрос единиц измерения объёма, массы")

	response, err = tm3.runIO([]byte{tm3.number, 0x03, 0xED, 0x02, 0x00, 0x01})
	for err != nil {
		return err
	}

	unitV := int(toWord([2]byte{response[0], response[1]}))

	switch unitV {
	case 0:
		{
			tm3.coefficientV = 1.0
			break
		}
	case 1:
		{
			tm3.coefficientV = 0.001
			break
		}
	default:
		{
			tm3.logger.Debug("Ошибка при расшифровке ед. измерения воды: %d ", unitV)
			return errors.New("не определены единицы измерения воды")
		}
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

	for i := 0; i < len(tm3.data.Systems); i++ {
		tm3.logger.Info("Запрос данных для системы %d", i+1)
		system, err := tm3.setSystemData(tm3.data.Systems[i], response, i)
		if err != nil {
			return &tm3.data, err
		}

		tm3.data.Systems[i] = *system
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

func (tm3 *TM3) initSerial(year uint16, month uint16, ef07 uint16, response []byte) string {
	serial := strconv.FormatUint(uint64(year), 10)
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

	return serial
}

func (tm3 *TM3) setSystemData(system models.SystemDevice, response []byte, systemNumber int) (*models.SystemDevice, error) {
	system.Status = true

	response, err := tm3.runIO([]byte{tm3.number, 0x03, 0x70, byte(systemNumber * 4), 0x00, 0x3A})
	for err != nil {
		return nil, err
	}

	system.SigmaQ = float64(float32(toDouble(response[0:8]) / 1000000))

	system.Q1 = float64(float32(toDouble(response[8:16]) / 1000000))
	system.M1 = float64(float32(toDouble(response[16:24])) * 0.001)
	system.GM1 = calculateFloatByPointer(response, 24) * 0.001
	system.GV1 = calculateFloatByPointer(response, 28) * tm3.coefficientV
	system.T1 = calculateFloatByPointer(response, 32)
	system.P1 = calculateFloatByPointer(response, 36) * tm3.coefficientP

	system.Q2 = float64(float32(toDouble(response[40:48]) / 1000000))
	system.M2 = float64(float32(toDouble(response[48:56]) * 0.001))
	system.GM2 = calculateFloatByPointer(response, 56) * 0.001
	system.GV2 = calculateFloatByPointer(response, 60) * tm3.coefficientV
	system.T2 = calculateFloatByPointer(response, 64)
	system.P2 = calculateFloatByPointer(response, 68) * tm3.coefficientP

	system.Q3 = float64(float32(toDouble(response[72:80]) / 1000000))
	system.T3 = calculateFloatByPointer(response, 104)
	system.P3 = calculateFloatByPointer(response, 108) * tm3.coefficientP
	system.TimeRunSys = calculateLongByPointer(response, 112)

	return &system, nil
}
