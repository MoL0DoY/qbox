package drivers

import (
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

/*
Драйвер согласно протоколу ТЭМ-104М-1 от 2017-11-18 - 2018-11-12
*/
type TEM104M1 struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

// Реализация интерфейса IDeviceDriver::Init
func (tem *TEM104M1) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Инициализация прибора, № %d", tem.counterNumber)
	tem.logger.Info("Читаем заводской номер")
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x00, 0x00, 0x04}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.SecondsReadTimeout = 5
	request.ControlFunction = tem.checkFrame
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	tem.data.AddNewSystem(1)
	tem.data.Systems[0].Status = true
	tem.data.UnitQ = models.Gcal

	tem.data.Serial = strconv.FormatUint(uint64(calculateLongByPointerLittleEndian(response, 0x06)), 10)
	tem.logger.Debug("Байты заводского номера (%s) - %X", tem.data.Serial, response[6:6+4])
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem *TEM104M1) Read() (*models.DataDevice, error) {

	tem.data.TimeRequest = time.Now()

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Получение даты времени на теплосчётчике")
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x02, 0x02, 0x00, 0x06}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	year := 2000 + int(response[11])
	month := time.Month(int(response[10]))
	day := int(response[9])
	hour := int(response[8])
	min := int(response[7])
	sek := int(response[6])
	tem.data.Time = time.Date(year, month, day, hour, min, sek, 0, time.Local)

	tem.logger.Info("Чтение оперативной памяти")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03, 0x00, 0x00, 0x28}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].GV1 = calculateFloatByPointerLittleEndian(response, 0x06+0x20)
	tem.data.Systems[0].GM1 = calculateFloatByPointerLittleEndian(response, 0x06+0x24)

	tem.logger.Info("Чтение интеграторов")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x01, 0x80, 0x51}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].SigmaQ = float64(float32(calculateLongByPointerLittleEndian(response, 0x06+0x10)) + calculateFloatByPointerLittleEndian(response, 0x06+0x20))
	tem.data.Systems[0].V1 = float64(float32(calculateLongByPointerLittleEndian(response, 0x06+0x08)) + calculateFloatByPointerLittleEndian(response, 0x06+0x18))
	tem.data.Systems[0].M1 = float64(float32(calculateLongByPointerLittleEndian(response, 0x06+0x0C)) + calculateFloatByPointerLittleEndian(response, 0x06+0x1C))
	tem.data.Systems[0].T1 = float32(toWord([2]byte{response[0x06+0x4B+0x01], response[0x06+0x4B]})) / 100
	tem.data.Systems[0].T2 = float32(toWord([2]byte{response[0x06+0x4B+0x03], response[0x06+0x4B+0x02]})) / 100
	tem.data.Systems[0].P1 = float32(response[0x06+0x4F]) / 100
	tem.data.Systems[0].P2 = float32(response[0x06+0x50]) / 100

	tem.data.TimeOn = calculateLongByPointerLittleEndian(response, 0x06+0x28)
	tem.data.Systems[0].TimeRunSys = calculateLongByPointerLittleEndian(response, 0x06+0x30)

	return &tem.data, nil
}

/**
* Проверка контрольной суммы
 */
func (*TEM104M1) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}

func (tem *TEM104M1) checkFrame(response []byte) bool {
	if len(response) < 6 {
		tem.logger.Info("Получено меньше 6 байт")
		return false
	}

	if response[0] != 0xAA || ((^response[1])&0xFF) != response[2] {
		tem.logger.Info("Заголовок ответа не верный: %X", response[0:5])
		return false
	}

	if len(response) < 6+int(response[5])+1 {
		tem.logger.Info("Размер полученных данных меньше ожидаемого: %X", 6+int(response[5])+1)
		return false
	}

	checkSum := response[len(response)-1]
	calculatedCheckSum := tem.calculateCheckSum(response[:len(response)-1])
	if calculatedCheckSum != checkSum {
		tem.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		tem.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		tem.logger.Debug("Получена контрольная сумма- %X", checkSum)
		return false
	}

	return true
}
