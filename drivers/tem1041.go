package drivers

import (
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

/*
Драйвер согласно протоколу ТЭМ-104-1
*/
type Tem104s1 struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

// Реализация интерфейса IDeviceDriver::Init
func (tem *Tem104s1) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Инициализация прибора, № %d", tem.counterNumber)
	tem.logger.Info("Читаем заводской номер")
	// 55-начало, (01 FE)-адрес, 0F-команда чтения 2К, 03-число посылаемых байт, (00 00)-нач.адрес, 07-длина получения
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x00, 0x00, 0x07}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.SecondsReadTimeout = 5
	request.ControlFunction = func(response []byte) bool {
		// ответ должен содержать заголовок в 6 байт, данные размером как запрашивали и контрольную сумму.
		if len(response) < 13 {
			tem.logger.Info("Ответ от прибора некорректный")
			return false
		}
		return true
	}
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	tem.data.AddNewSystem(1)
	tem.data.Systems[0].Status = true
	tem.data.UnitQ = models.Gcal

	tem.data.Serial = string(response[6:13])
	tem.logger.Debug("Заводской номер - %s", tem.data.Serial)

	return nil
}

func (tem *Tem104s1) Read() (*models.DataDevice, error) {

	tem.data.TimeRequest = time.Now()

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Получение даты времени на теплосчётчике")
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x02, 0x02, 0x00, 0x07}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}
	year := 2000 + DecodeBcd([]byte{response[12]})
	month := time.Month(DecodeBcd([]byte{response[11]}))
	day := DecodeBcd([]byte{response[10]})
	hour := DecodeBcd([]byte{response[8]})
	min := DecodeBcd([]byte{response[7]})
	sek := DecodeBcd([]byte{response[6]})
	tem.data.Time = time.Date(year, month, day, hour, min, sek, 0, time.Local)

	tem.logger.Info("Чтение оперативной памяти")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03, 0x00, 0xB8, 0x18}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].GV1 = calculateFloatByPointer(response, 0x06)
	tem.data.Systems[0].GM1 = calculateFloatByPointer(response, 0x06+0x04)
	tem.data.Systems[0].T1 = calculateFloatByPointer(response, 0x06+0x08)
	tem.data.Systems[0].T2 = calculateFloatByPointer(response, 0x06+0x08+0x04)
	tem.data.Systems[0].P1 = calculateFloatByPointer(response, 0x06+0x08+0x08)
	tem.data.Systems[0].P2 = calculateFloatByPointer(response, 0x06+0x08+0x08+0x04)

	tem.logger.Info("Чтение интеграторов")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x01, 0x44, 0x20}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].SigmaQ = float64(float32(calculateLongByPointer(response, 0x06+0x10)) + calculateFloatByPointer(response, 0x06+0x14))
	tem.data.Systems[0].V1 = float64(float32(calculateLongByPointer(response, 0x06)) + calculateFloatByPointer(response, 0x06+0x04))
	tem.data.Systems[0].M1 = float64(float32(calculateLongByPointer(response, 0x06+0x08)) + calculateFloatByPointer(response, 0x06+0x0C))

	tem.data.TimeOn = calculateLongByPointer(response, 0x06+0x18)
	tem.data.TimeRunCommon = calculateLongByPointer(response, 0x06+0x1C)

	return &tem.data, nil
}

/**
* Проверка контрольной суммы
 */
func (*Tem104s1) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}

func (tem *Tem104s1) checkFrame(response []byte) bool {
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
